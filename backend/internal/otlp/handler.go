package otlp

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/google/uuid"
)

// SystemEventInserter is satisfied by repository.SystemEventRepo.
type SystemEventInserter interface {
	InsertBatch(ctx context.Context, events []model.SystemEvent) (int, error)
}

// OTLPHandler serves the OTLP HTTP/JSON endpoints for traces and logs.
type OTLPHandler struct {
	repo SystemEventInserter
}

// NewOTLPHandler creates an OTLPHandler with the given repository.
func NewOTLPHandler(repo SystemEventInserter) *OTLPHandler {
	return &OTLPHandler{repo: repo}
}

// --- OTLP JSON types (minimal, inline) ---

type otlpAnyValue struct {
	StringValue *string        `json:"stringValue,omitempty"`
	IntValue    *string        `json:"intValue,omitempty"`
	BoolValue   *bool          `json:"boolValue,omitempty"`
	DoubleValue *float64       `json:"doubleValue,omitempty"`
	ArrayValue  *otlpArrayVal  `json:"arrayValue,omitempty"`
	KvlistValue *otlpKvlistVal `json:"kvlistValue,omitempty"`
}

type otlpArrayVal struct {
	Values []otlpAnyValue `json:"values"`
}

type otlpKvlistVal struct {
	Values []otlpKeyValue `json:"values"`
}

type otlpKeyValue struct {
	Key   string       `json:"key"`
	Value otlpAnyValue `json:"value"`
}

type otlpResource struct {
	Attributes []otlpKeyValue `json:"attributes"`
}

// --- Traces types ---

type exportTraceServiceRequest struct {
	ResourceSpans []resourceSpan `json:"resourceSpans"`
}

type resourceSpan struct {
	Resource   otlpResource `json:"resource"`
	ScopeSpans []scopeSpan  `json:"scopeSpans"`
}

type scopeSpan struct {
	Spans []otlpSpan `json:"spans"`
}

type otlpSpan struct {
	TraceID           string         `json:"traceId"`
	SpanID            string         `json:"spanId"`
	ParentSpanID      string         `json:"parentSpanId"`
	Name              string         `json:"name"`
	StartTimeUnixNano string         `json:"startTimeUnixNano"`
	EndTimeUnixNano   string         `json:"endTimeUnixNano"`
	Status            otlpStatus     `json:"status"`
	Attributes        []otlpKeyValue `json:"attributes"`
}

type otlpStatus struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// --- Logs types ---

type exportLogsServiceRequest struct {
	ResourceLogs []resourceLog `json:"resourceLogs"`
}

type resourceLog struct {
	Resource  otlpResource `json:"resource"`
	ScopeLogs []scopeLog   `json:"scopeLogs"`
}

type scopeLog struct {
	LogRecords []otlpLogRecord `json:"logRecords"`
}

type otlpLogRecord struct {
	TimeUnixNano   string         `json:"timeUnixNano"`
	SeverityNumber int            `json:"severityNumber"`
	SeverityText   string         `json:"severityText"`
	Body           otlpAnyValue   `json:"body"`
	TraceID        string         `json:"traceId"`
	SpanID         string         `json:"spanId"`
	Attributes     []otlpKeyValue `json:"attributes"`
}

// TracesHandler handles POST /v1/traces (OTLP JSON).
func (h *OTLPHandler) TracesHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeOTLPError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	if len(body) == 0 {
		writeOTLPSuccess(w)
		return
	}

	var req exportTraceServiceRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeOTLPError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	var events []model.SystemEvent

	for _, rs := range req.ResourceSpans {
		serviceName := extractServiceName(rs.Resource)
		resourceJSON := kvToJSON(rs.Resource.Attributes)

		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				traceID := normalizeHexID(span.TraceID)
				spanID := normalizeHexID(span.SpanID)
				parentSpanID := normalizeHexID(span.ParentSpanID)

				severity := spanStatusToSeverity(span.Status.Code)
				eventName := span.Name
				durationMs := computeDurationMs(span.StartTimeUnixNano, span.EndTimeUnixNano)
				timestamp := nanoToTime(span.StartTimeUnixNano)
				attrsJSON := kvToJSON(span.Attributes)

				ev := model.SystemEvent{
					EventID:       uuid.New(),
					Timestamp:     timestamp,
					TraceID:       strPtr(traceID),
					SpanID:        strPtr(spanID),
					ParentSpanID:  nilIfEmpty(parentSpanID),
					SourceType:    "otlp",
					SourceService: serviceName,
					Severity:      severity,
					EventName:     strPtr(eventName),
					Attributes:    attrsJSON,
					Resource:      resourceJSON,
					DurationMs:    durationMs,
				}
				events = append(events, ev)
			}
		}
	}

	if len(events) == 0 {
		writeOTLPSuccess(w)
		return
	}

	inserted, err := h.repo.InsertBatch(r.Context(), events)
	if err != nil {
		slog.Error("otlp traces insert failed", "error", err)
		writeOTLPError(w, http.StatusInternalServerError, "insert failed")
		return
	}

	slog.Info("otlp traces ingested", "spans", len(events), "inserted", inserted)
	writeOTLPSuccess(w)
}

// LogsHandler handles POST /v1/logs (OTLP JSON).
func (h *OTLPHandler) LogsHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeOTLPError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	if len(body) == 0 {
		writeOTLPSuccess(w)
		return
	}

	var req exportLogsServiceRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeOTLPError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	var events []model.SystemEvent

	for _, rl := range req.ResourceLogs {
		serviceName := extractServiceName(rl.Resource)
		resourceJSON := kvToJSON(rl.Resource.Attributes)

		for _, sl := range rl.ScopeLogs {
			for _, rec := range sl.LogRecords {
				traceID := normalizeHexID(rec.TraceID)
				spanID := normalizeHexID(rec.SpanID)

				severity := logSeverityNumberToString(rec.SeverityNumber)
				timestamp := nanoToTime(rec.TimeUnixNano)
				attrsJSON := kvToJSON(rec.Attributes)
				message := anyValueToString(&rec.Body)

				ev := model.SystemEvent{
					EventID:       uuid.New(),
					Timestamp:     timestamp,
					TraceID:       nilIfEmpty(traceID),
					SpanID:        nilIfEmpty(spanID),
					SourceType:    "otlp",
					SourceService: serviceName,
					Severity:      severity,
					Message:       nilIfEmpty(message),
					Attributes:    attrsJSON,
					Resource:      resourceJSON,
				}
				events = append(events, ev)
			}
		}
	}

	if len(events) == 0 {
		writeOTLPSuccess(w)
		return
	}

	inserted, err := h.repo.InsertBatch(r.Context(), events)
	if err != nil {
		slog.Error("otlp logs insert failed", "error", err)
		writeOTLPError(w, http.StatusInternalServerError, "insert failed")
		return
	}

	slog.Info("otlp logs ingested", "records", len(events), "inserted", inserted)
	writeOTLPSuccess(w)
}

// --- helpers ---

func extractServiceName(res otlpResource) string {
	for _, kv := range res.Attributes {
		if kv.Key == "service.name" {
			return anyValueToString(&kv.Value)
		}
	}
	return "unknown"
}

func spanStatusToSeverity(code int) string {
	switch code {
	case 2:
		return "error"
	default:
		return "info"
	}
}

func logSeverityNumberToString(n int) string {
	switch {
	case n >= 17:
		return "fatal"
	case n >= 13:
		return "error"
	case n >= 9:
		return "warn"
	case n >= 5:
		return "info"
	case n >= 1:
		return "debug"
	default:
		return "info"
	}
}

func computeDurationMs(startNano, endNano string) *int {
	s, err1 := strconv.ParseInt(startNano, 10, 64)
	e, err2 := strconv.ParseInt(endNano, 10, 64)
	if err1 != nil || err2 != nil || e <= s {
		return nil
	}
	ms := int((e - s) / 1e6)
	return &ms
}

func nanoToTime(nanos string) time.Time {
	n, err := strconv.ParseInt(nanos, 10, 64)
	if err != nil || n == 0 {
		return time.Now().UTC()
	}
	return time.Unix(0, n).UTC()
}

// normalizeHexID decodes hex bytes and re-encodes to ensure lowercase consistency.
func normalizeHexID(s string) string {
	if s == "" {
		return ""
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return s
	}
	return hex.EncodeToString(b)
}

func kvToJSON(kvs []otlpKeyValue) json.RawMessage {
	if len(kvs) == 0 {
		return json.RawMessage("{}")
	}
	m := make(map[string]any, len(kvs))
	for _, kv := range kvs {
		m[kv.Key] = anyValueToGo(&kv.Value)
	}
	b, err := json.Marshal(m)
	if err != nil {
		return json.RawMessage("{}")
	}
	return b
}

func anyValueToGo(v *otlpAnyValue) any {
	if v == nil {
		return nil
	}
	if v.StringValue != nil {
		return *v.StringValue
	}
	if v.IntValue != nil {
		n, _ := strconv.ParseInt(*v.IntValue, 10, 64)
		return n
	}
	if v.BoolValue != nil {
		return *v.BoolValue
	}
	if v.DoubleValue != nil {
		return *v.DoubleValue
	}
	if v.ArrayValue != nil {
		arr := make([]any, len(v.ArrayValue.Values))
		for i := range v.ArrayValue.Values {
			arr[i] = anyValueToGo(&v.ArrayValue.Values[i])
		}
		return arr
	}
	if v.KvlistValue != nil {
		m := make(map[string]any, len(v.KvlistValue.Values))
		for _, kv := range v.KvlistValue.Values {
			m[kv.Key] = anyValueToGo(&kv.Value)
		}
		return m
	}
	return nil
}

func anyValueToString(v *otlpAnyValue) string {
	if v == nil {
		return ""
	}
	if v.StringValue != nil {
		return *v.StringValue
	}
	if v.IntValue != nil {
		return *v.IntValue
	}
	if v.BoolValue != nil {
		return fmt.Sprintf("%v", *v.BoolValue)
	}
	if v.DoubleValue != nil {
		return fmt.Sprintf("%g", *v.DoubleValue)
	}
	return ""
}

func strPtr(s string) *string {
	return &s
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func writeOTLPSuccess(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{}"))
}

func writeOTLPError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
