package model

import (
	"strings"
	"testing"
)

func TestValidateAgentID(t *testing.T) {
	t.Run("valid IDs", func(t *testing.T) {
		valid := []string{
			"claude",
			"claude-3",
			"my_agent",
			"agent-1_v2",
			"A",
			"a",
			"0",
			strings.Repeat("a", 128),
			"UPPER_CASE",
			"mix-ED_123",
		}
		for _, id := range valid {
			if err := ValidateAgentID(id); err != nil {
				t.Errorf("expected valid agent ID %q to pass, got error: %v", id, err)
			}
		}
	})

	t.Run("invalid IDs", func(t *testing.T) {
		invalid := []string{
			"",
			strings.Repeat("a", 129),
			"has space",
			"has.dot",
			"has@at",
			"has/slash",
			"has:colon",
			"has!bang",
		}
		for _, id := range invalid {
			if err := ValidateAgentID(id); err == nil {
				t.Errorf("expected invalid agent ID %q to fail, but got no error", id)
			}
		}
	})
}
