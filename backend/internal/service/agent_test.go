package service

import (
	"context"
	"testing"
)

func TestAgentService_EnsureExists_InvalidID(t *testing.T) {
	// nil repo is intentional: validation must fail before any DB call is made.
	svc := NewAgentService(nil)

	invalid := []string{
		"",
		"has space",
		"has.dot",
		"has@at",
		"has/slash",
	}

	for _, id := range invalid {
		t.Run(id, func(t *testing.T) {
			err := svc.EnsureExists(context.Background(), id)
			if err == nil {
				t.Errorf("expected error for invalid agent ID %q, got nil", id)
			}
		})
	}
}

func TestAgentService_EnsureExists_ValidID_CallsRepo(t *testing.T) {
	called := false
	repo := &mockAgentRepo{
		upsertFn: func(ctx context.Context, agentID string) error {
			called = true
			return nil
		},
	}

	svc := NewAgentService(repo)
	if err := svc.EnsureExists(context.Background(), "claude-agent"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected repo.Upsert to be called for a valid agent ID")
	}
}

// mockAgentRepo satisfies the agentRepo interface for testing.
type mockAgentRepo struct {
	upsertFn func(ctx context.Context, agentID string) error
}

func (m *mockAgentRepo) Upsert(ctx context.Context, agentID string) error {
	return m.upsertFn(ctx, agentID)
}
