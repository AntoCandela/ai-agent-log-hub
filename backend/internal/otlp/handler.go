// Package otlp implements an OpenTelemetry Protocol (OTLP) receiver that
// accepts trace and log data from any OpenTelemetry-instrumented service and
// converts it into system events stored in the ai-agent-log-hub database.
//
// OpenTelemetry (OTel) is an industry-standard observability framework. Services
// instrumented with OTel emit telemetry in OTLP format — a structured wire
// format containing traces (distributed request timelines) and logs. This
// package handles the HTTP/JSON variant of OTLP.
//
// The key conversion this package performs is "span-to-event": each incoming
// OTLP span (a single unit of work in a distributed trace) is mapped to one
// system_events row. This lets the log-hub correlate infrastructure telemetry
// with agent sessions via shared trace IDs (see repository.SystemEventRepo.LinkToSessions).
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

// SystemEventInserter is satisfied by repository.SystemEventRepo. Using an
// interface here decouples the OTLP handler from the concrete repository type.
type SystemEventInserter interface {
	InsertBatch(ctx context.Context, events []model.SystemEvent) (int, error)
}

// OTLPHandler serves the OTLP HTTP/JSON endpoints for traces and logs.
// It receives standard OTLP payloads, converts spans/logs to system events,
// and stores them in the database for later correlation with agent sessions.
type OTLPHandler struct {
	repo SystemEventInserter
}

// NewOTLPHandler creates an OTLPHandler with the given repository.
func NewOTLPHandler(repo SystemEventInserter) *OTLPHandler {
	return &OTLPHandler{repo: repo}
}

// ---------------------------------------------------------------------------
// OTLP JSON types (minimal, inline)
//
// These structs mirror the OTLP JSON wire format. They only include fields
// this handler actually reads — many optional OTLP fields are omitted.
// ---------------------------------------------------------------------------

// otlpAnyValue represents a value in the OTLP type system. In OTLP, attribute
// values can be strings, ints, bools, doubles, arrays, or nested key-value
// lists. Exactly one of these fields will be non-nil.
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
//
// Span-to-event conversion:
//   Each OTLP span is converted to one model.SystemEvent:
//   - trace_id and span_id are normalized to lowercase hex for consistency.
//   - The span name becomes the event_name.
//   - The span status code is mapped to a severity string (error vs info).
//   - Duration is computed from start/end nanosecond timestamps.
//   - Span attributes and resource attributes are stored as JSON.
//   - source_type is always "otlp"; source_service comes from the resource's
//     "service.name" attribute.
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
//
// Each OTLP log record is converted to one model.SystemEvent. The log body
// (which can be any OTLP value type) is converted to a string for the message
// field. Severity is mapped from the numeric OTLP severity level to a string
// (debug/info/warn/error/fatal).
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

				severity := LogSeverityNumberToString(rec.SeverityNumber)
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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// extractServiceName looks for the well-known "service.name" attribute in the
// OTLP resource. This attribute identifies which service emitted the telemetry.
func extractServiceName(res otlpResource) string {
	for _, kv := range res.Attributes {
		if kv.Key == "service.name" {
			return anyValueToString(&kv.Value)
		}
	}
	return "unknown"
}

// spanStatusToSeverity maps the OTLP span status code to a severity string.
// Code 2 means ERROR in the OTLP spec; everything else is treated as info.
func spanStatusToSeverity(code int) string {
	switch code {
	case 2:
		return "error"
	default:
		return "info"
	}
}

// LogSeverityNumberToString maps OTLP severity numbers to string levels.
func LogSeverityNumberToString(n int) string {
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

// computeDurationMs calculates the span duration in milliseconds from two
// nanosecond-precision timestamp strings. Returns nil if either timestamp is
// invalid or the span has zero/negative duration.
func computeDurationMs(startNano, endNano string) *int {
	s, err1 := strconv.ParseInt(startNano, 10, 64)
	e, err2 := strconv.ParseInt(endNano, 10, 64)
	if err1 != nil || err2 != nil || e <= s {
		return nil
	}
	ms := int((e - s) / 1e6)
	return &ms
}

// nanoToTime converts a nanosecond Unix timestamp string to a Go time.Time.
// Falls back to the current time if the string is empty or invalid.
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

// kvToJSON converts OTLP key-value attributes into a JSON object (stored as
// json.RawMessage for direct insertion into a JSONB column).
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

// anyValueToGo converts an OTLP typed value into a plain Go value for JSON
// marshaling (string, int64, bool, float64, []any, or map[string]any).
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

// anyValueToString converts an OTLP typed value to its string representation.
// Used for extracting human-readable messages from log record bodies.
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

// strPtr returns a pointer to s. Go does not allow taking the address of a
// string literal directly, so this helper is needed for populating *string fields.
func strPtr(s string) *string {
	return &s
}

// nilIfEmpty returns nil if s is empty, otherwise a pointer to s. This is used
// to store empty strings as SQL NULL rather than an empty string.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// writeOTLPSuccess writes an empty JSON object with status 200, which is the
// standard OTLP success response.
func writeOTLPSuccess(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{}"))
}

// writeOTLPError writes a JSON error response with the given HTTP status code.
func writeOTLPError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
