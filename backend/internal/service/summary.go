package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/embed"
	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/repository"
	"github.com/google/uuid"
)

// summaryEventQuerier is the subset of AgentEventRepo that the summary service needs.
type summaryEventQuerier interface {
	Query(ctx context.Context, filters repository.EventFilters) ([]model.AgentEvent, int, error)
}

// summaryStore is the subset of SummaryRepo that the summary service needs.
type summaryStore interface {
	Create(ctx context.Context, summary *model.SessionSummary) error
}

// embeddingStore is the subset of EmbeddingRepo that the summary service needs.
type embeddingStore interface {
	Store(ctx context.Context, sourceType string, sourceID uuid.UUID, agentID, content string, embedding []float32, shared bool) error
}

// SummaryService generates session summaries from event data.
type SummaryService struct {
	summaryRepo summaryStore
	eventRepo   summaryEventQuerier
	embedRepo   embeddingStore
	embedder    embed.Embedder
}

// NewSummaryService creates a SummaryService with the given dependencies.
func NewSummaryService(
	summaryRepo summaryStore,
	eventRepo summaryEventQuerier,
	embedRepo embeddingStore,
	embedder embed.Embedder,
) *SummaryService {
	return &SummaryService{
		summaryRepo: summaryRepo,
		eventRepo:   eventRepo,
		embedRepo:   embedRepo,
		embedder:    embedder,
	}
}

// fileInfo tracks per-file modification counts.
type fileInfo struct {
	FilePath string `json:"file_path"`
	Changes  int    `json:"changes"`
}

// toolInfo tracks per-tool usage counts.
type toolInfo struct {
	ToolName string `json:"tool_name"`
	Count    int    `json:"count"`
}

// commitInfo holds commit hash and message.
type commitInfo struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
}

// errorInfo holds an error event summary.
type errorInfo struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// timelineEntry holds a key event in the session timeline.
type timelineEntry struct {
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"`
	Detail    string    `json:"detail"`
}

// GenerateForSession aggregates events for the given session and stores a summary
// with an optional embedding.
func (s *SummaryService) GenerateForSession(ctx context.Context, session *model.Session) error {
	// 1. Query all events for this session (up to 10 000).
	sid := session.SessionID
	events, total, err := s.eventRepo.Query(ctx, repository.EventFilters{
		SessionID: &sid,
		Limit:     10000,
		Order:     "asc",
	})
	if err != nil {
		return fmt.Errorf("SummaryService.GenerateForSession query events: %w", err)
	}

	// 2. Aggregate data from events.
	filesMap := make(map[string]int)
	toolsMap := make(map[string]int)
	var commits []commitInfo
	var errors []errorInfo
	var timeline []timelineEntry

	for _, e := range events {
		// Files modified: extract file_path from params.
		if e.Params != nil {
			var params map[string]any
			if json.Unmarshal(e.Params, &params) == nil {
				if fp, ok := params["file_path"].(string); ok && fp != "" {
					filesMap[fp]++
				}
			}
		}

		// Tools used: count by tool_name.
		if e.ToolName != nil && *e.ToolName != "" {
			toolsMap[*e.ToolName]++
		}

		// Commits: filter event_type == "git_commit".
		if e.EventType == "git_commit" {
			ci := commitInfo{}
			if e.Params != nil {
				var params map[string]any
				if json.Unmarshal(e.Params, &params) == nil {
					if h, ok := params["hash"].(string); ok {
						ci.Hash = h
					}
					if m, ok := params["message"].(string); ok {
						ci.Message = m
					}
				}
			}
			if ci.Hash == "" && e.Message != nil {
				ci.Message = *e.Message
			}
			commits = append(commits, ci)

			timeline = append(timeline, timelineEntry{
				Timestamp: e.Timestamp,
				EventType: "commit",
				Detail:    ci.Message,
			})
		}

		// Errors: filter severity == "error".
		if e.Severity == "error" {
			msg := ""
			if e.Message != nil {
				msg = *e.Message
			}
			errors = append(errors, errorInfo{
				Message:   msg,
				Timestamp: e.Timestamp,
			})

			timeline = append(timeline, timelineEntry{
				Timestamp: e.Timestamp,
				EventType: "error",
				Detail:    msg,
			})
		}
	}

	// Add first and last event to timeline if we have events.
	if len(events) > 0 {
		first := events[0]
		firstDetail := first.EventType
		if first.ToolName != nil {
			firstDetail = *first.ToolName
		}
		timeline = append(timeline, timelineEntry{
			Timestamp: first.Timestamp,
			EventType: "session_start",
			Detail:    firstDetail,
		})
	}

	// Sort timeline by timestamp.
	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Timestamp.Before(timeline[j].Timestamp)
	})

	// Build JSON fields.
	files := make([]fileInfo, 0, len(filesMap))
	for fp, count := range filesMap {
		files = append(files, fileInfo{FilePath: fp, Changes: count})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Changes > files[j].Changes })

	tools := make([]toolInfo, 0, len(toolsMap))
	for tn, count := range toolsMap {
		tools = append(tools, toolInfo{ToolName: tn, Count: count})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Count > tools[j].Count })

	filesJSON, _ := json.Marshal(files)
	toolsJSON, _ := json.Marshal(tools)
	commitsJSON, _ := json.Marshal(commits)
	errorsJSON, _ := json.Marshal(errors)
	timelineJSON, _ := json.Marshal(timeline)

	// 3. Compute duration.
	durationSec := 0
	if session.EndedAt != nil {
		durationSec = int(session.EndedAt.Sub(session.StartedAt).Seconds())
	} else if len(events) > 0 {
		durationSec = int(events[len(events)-1].Timestamp.Sub(events[0].Timestamp).Seconds())
	}

	// 4. Generate summary text.
	durationMin := durationSec / 60
	summaryText := fmt.Sprintf(
		"Session lasted %dm. Modified %d files, used %d tools. %d commits. %d errors.",
		durationMin, len(files), len(tools), len(commits), len(errors),
	)

	// 5. Store in session_summaries.
	summary := &model.SessionSummary{
		SessionID:       session.SessionID,
		AgentID:         session.AgentID,
		DurationSeconds: durationSec,
		EventCount:      total,
		FilesModified:   filesJSON,
		ToolsUsed:       toolsJSON,
		Commits:         commitsJSON,
		Errors:          errorsJSON,
		Timeline:        timelineJSON,
		SummaryText:     &summaryText,
	}

	if err := s.summaryRepo.Create(ctx, summary); err != nil {
		return fmt.Errorf("SummaryService.GenerateForSession store: %w", err)
	}

	// 6. Embed summary_text and store in embeddings table.
	if s.embedder != nil && s.embedRepo != nil {
		vec, err := s.embedder.Embed(ctx, summaryText)
		if err != nil {
			// Log but don't fail the summary generation for embedding errors.
			slog.Warn("SummaryService: failed to embed summary", "error", err, "session_id", session.SessionID)
		} else {
			if err := s.embedRepo.Store(ctx, "session_summary", summary.ID, session.AgentID, summaryText, vec, false); err != nil {
				slog.Warn("SummaryService: failed to store embedding", "error", err, "session_id", session.SessionID)
			}
		}
	}

	return nil
}
