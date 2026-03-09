package workspace

import (
	"strings"
	"testing"
)

func TestSummary(t *testing.T) {
	w := &Workspace{
		ID:          "ws_12345_1",
		Name:        "frontend-project",
		Sessions:    []string{"morning-tabs", "afternoon-tabs"},
		Collections: []string{"ui-references", "api-docs", "notes"},
		CreatedAt:   "2026-03-07T10:00:00Z",
		UpdatedAt:   "2026-03-07T12:00:00Z",
	}

	s := w.Summary()

	if s.ID != w.ID {
		t.Errorf("ID = %q, want %q", s.ID, w.ID)
	}
	if s.Name != w.Name {
		t.Errorf("Name = %q, want %q", s.Name, w.Name)
	}
	if s.SessionCount != 2 {
		t.Errorf("SessionCount = %d, want 2", s.SessionCount)
	}
	if s.CollectionCount != 3 {
		t.Errorf("CollectionCount = %d, want 3", s.CollectionCount)
	}
	if s.CreatedAt != w.CreatedAt {
		t.Errorf("CreatedAt = %q, want %q", s.CreatedAt, w.CreatedAt)
	}
	if s.UpdatedAt != w.UpdatedAt {
		t.Errorf("UpdatedAt = %q, want %q", s.UpdatedAt, w.UpdatedAt)
	}
}

func TestSummaryEmpty(t *testing.T) {
	w := &Workspace{
		ID:          "ws_12345_2",
		Name:        "empty",
		Sessions:    []string{},
		Collections: []string{},
		CreatedAt:   "2026-03-07T10:00:00Z",
		UpdatedAt:   "2026-03-07T10:00:00Z",
	}

	s := w.Summary()
	if s.SessionCount != 0 {
		t.Errorf("SessionCount = %d, want 0", s.SessionCount)
	}
	if s.CollectionCount != 0 {
		t.Errorf("CollectionCount = %d, want 0", s.CollectionCount)
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()

	if !strings.HasPrefix(id1, "ws_") {
		t.Errorf("id should start with ws_, got %q", id1)
	}
	if id1 == id2 {
		t.Error("ids should be unique")
	}
}

func TestGenerateIDUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateID()
		if seen[id] {
			t.Fatalf("duplicate id at iteration %d: %s", i, id)
		}
		seen[id] = true
	}
}
