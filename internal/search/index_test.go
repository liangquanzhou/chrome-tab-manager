package search

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSearchIndex(t *testing.T) {
	si := NewSearchIndex("/tmp/test-index.json")
	if si == nil {
		t.Fatal("NewSearchIndex returned nil")
	}
	if si.Len() != 0 {
		t.Errorf("new index should be empty, got %d entries", si.Len())
	}
}

func TestUpsertAndSearch(t *testing.T) {
	si := NewSearchIndex("")

	si.Upsert(&IndexEntry{
		Kind:      "session",
		ID:        "work-session",
		Title:     "work-session",
		UpdatedAt: time.Now(),
	})
	si.Upsert(&IndexEntry{
		Kind:      "collection",
		ID:        "reading-list",
		Title:     "reading-list",
		URL:       "https://example.com",
		UpdatedAt: time.Now(),
	})

	if si.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", si.Len())
	}

	// Search for "work"
	results := si.Search("work", nil)
	if len(results) != 1 {
		t.Fatalf("search 'work': expected 1 result, got %d", len(results))
	}
	if results[0].ID != "work-session" {
		t.Errorf("expected ID 'work-session', got %q", results[0].ID)
	}

	// Search for "reading" - should match the collection
	results = si.Search("reading", nil)
	if len(results) != 1 {
		t.Fatalf("search 'reading': expected 1 result, got %d", len(results))
	}
	if results[0].Kind != "collection" {
		t.Errorf("expected kind 'collection', got %q", results[0].Kind)
	}

	// Search with empty query matches all
	results = si.Search("", nil)
	if len(results) != 2 {
		t.Errorf("empty query: expected 2 results, got %d", len(results))
	}
}

func TestSearchFilterByKinds(t *testing.T) {
	si := NewSearchIndex("")

	si.Upsert(&IndexEntry{Kind: "session", ID: "s1", Title: "project", UpdatedAt: time.Now()})
	si.Upsert(&IndexEntry{Kind: "collection", ID: "c1", Title: "project-links", UpdatedAt: time.Now()})
	si.Upsert(&IndexEntry{Kind: "workspace", ID: "w1", Title: "project-workspace", UpdatedAt: time.Now()})

	// Filter to sessions only
	results := si.Search("project", []string{"session"})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Kind != "session" {
		t.Errorf("expected kind 'session', got %q", results[0].Kind)
	}

	// Filter to collections + workspaces
	results = si.Search("project", []string{"collection", "workspace"})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestSearchByURL(t *testing.T) {
	si := NewSearchIndex("")

	si.Upsert(&IndexEntry{
		Kind:      "collection",
		ID:        "c1",
		Title:     "Links",
		URL:       "https://github.com/user/repo",
		UpdatedAt: time.Now(),
	})

	results := si.Search("github", nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "c1" {
		t.Errorf("expected ID 'c1', got %q", results[0].ID)
	}
}

func TestSearchByTags(t *testing.T) {
	si := NewSearchIndex("")

	si.Upsert(&IndexEntry{
		Kind:      "workspace",
		ID:        "w1",
		Title:     "My Workspace",
		Tags:      []string{"golang", "backend"},
		UpdatedAt: time.Now(),
	})

	results := si.Search("golang", nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "w1" {
		t.Errorf("expected ID 'w1', got %q", results[0].ID)
	}
}

func TestSearchNoMatch(t *testing.T) {
	si := NewSearchIndex("")

	si.Upsert(&IndexEntry{Kind: "session", ID: "s1", Title: "work", UpdatedAt: time.Now()})

	results := si.Search("zzzzz", nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchScoreOrdering(t *testing.T) {
	si := NewSearchIndex("")

	// Exact match should score higher than prefix, which scores higher than contains
	si.Upsert(&IndexEntry{Kind: "session", ID: "s1", Title: "github", UpdatedAt: time.Now()})
	si.Upsert(&IndexEntry{Kind: "session", ID: "s2", Title: "github-repos", UpdatedAt: time.Now()})
	si.Upsert(&IndexEntry{Kind: "session", ID: "s3", Title: "my-github-stuff", UpdatedAt: time.Now()})

	results := si.Search("github", nil)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Exact match first
	if results[0].ID != "s1" {
		t.Errorf("first result should be exact match 's1', got %q", results[0].ID)
	}
	// Prefix match second
	if results[1].ID != "s2" {
		t.Errorf("second result should be prefix match 's2', got %q", results[1].ID)
	}
	// Contains match third
	if results[2].ID != "s3" {
		t.Errorf("third result should be contains match 's3', got %q", results[2].ID)
	}
}

func TestUpsertOverwrites(t *testing.T) {
	si := NewSearchIndex("")

	si.Upsert(&IndexEntry{Kind: "session", ID: "s1", Title: "old-title", UpdatedAt: time.Now()})
	si.Upsert(&IndexEntry{Kind: "session", ID: "s1", Title: "new-title", UpdatedAt: time.Now()})

	if si.Len() != 1 {
		t.Errorf("expected 1 entry after upsert, got %d", si.Len())
	}

	results := si.Search("new-title", nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "new-title" {
		t.Errorf("title should be updated, got %q", results[0].Title)
	}
}

func TestUpsertNil(t *testing.T) {
	si := NewSearchIndex("")
	si.Upsert(nil) // should not panic
	if si.Len() != 0 {
		t.Errorf("nil upsert should be no-op, got %d entries", si.Len())
	}
}

func TestRemove(t *testing.T) {
	si := NewSearchIndex("")

	si.Upsert(&IndexEntry{Kind: "session", ID: "s1", Title: "work", UpdatedAt: time.Now()})
	si.Upsert(&IndexEntry{Kind: "collection", ID: "c1", Title: "links", UpdatedAt: time.Now()})

	si.Remove("session", "s1")

	if si.Len() != 1 {
		t.Errorf("expected 1 entry after remove, got %d", si.Len())
	}

	results := si.Search("work", nil)
	if len(results) != 0 {
		t.Errorf("removed entry should not appear in search")
	}
}

func TestRemoveNonexistent(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "idx.json")

	si := NewSearchIndex(indexPath)
	si.Upsert(&IndexEntry{Kind: "session", ID: "s1", Title: "work", UpdatedAt: time.Now()})

	// Save to clear dirty flag
	if err := si.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if si.IsDirty() {
		t.Fatal("should not be dirty after save")
	}

	si.Remove("session", "nonexistent")

	if si.Len() != 1 {
		t.Errorf("removing nonexistent entry should not affect count, got %d", si.Len())
	}
	if si.IsDirty() {
		t.Error("removing nonexistent entry should not mark index dirty")
	}
}

func TestRemoveByKind(t *testing.T) {
	si := NewSearchIndex("")

	si.Upsert(&IndexEntry{Kind: "session", ID: "s1", Title: "a", UpdatedAt: time.Now()})
	si.Upsert(&IndexEntry{Kind: "session", ID: "s2", Title: "b", UpdatedAt: time.Now()})
	si.Upsert(&IndexEntry{Kind: "collection", ID: "c1", Title: "c", UpdatedAt: time.Now()})

	si.RemoveByKind("session")

	if si.Len() != 1 {
		t.Errorf("expected 1 entry after RemoveByKind, got %d", si.Len())
	}

	results := si.Search("", []string{"session"})
	if len(results) != 0 {
		t.Errorf("no sessions should remain, got %d", len(results))
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "search_index.json")

	// Create and populate
	si := NewSearchIndex(indexPath)
	si.Upsert(&IndexEntry{
		Kind:      "session",
		ID:        "my-session",
		Title:     "My Work Session",
		UpdatedAt: time.Now(),
		Checksum:  "abc123",
	})
	si.Upsert(&IndexEntry{
		Kind:      "workspace",
		ID:        "ws_1",
		Title:     "Dev Workspace",
		Tags:      []string{"dev", "golang"},
		UpdatedAt: time.Now(),
		Checksum:  "def456",
	})

	if err := si.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if si.IsDirty() {
		t.Error("index should not be dirty after Save")
	}

	// Load into a new instance
	si2 := NewSearchIndex(indexPath)
	if err := si2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if si2.Len() != 2 {
		t.Fatalf("loaded index should have 2 entries, got %d", si2.Len())
	}

	results := si2.Search("Dev", nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "ws_1" {
		t.Errorf("expected ID 'ws_1', got %q", results[0].ID)
	}
	if len(results[0].Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(results[0].Tags))
	}
}

func TestSaveNoDirtyNoOp(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "search_index.json")

	si := NewSearchIndex(indexPath)
	// Save without any changes should be a no-op
	if err := si.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// File should not exist
	if _, err := os.Stat(indexPath); !os.IsNotExist(err) {
		t.Error("index file should not be created when nothing is dirty")
	}
}

func TestLoadNonexistent(t *testing.T) {
	si := NewSearchIndex("/tmp/nonexistent-ctm-index.json")
	if err := si.Load(); err != nil {
		t.Fatalf("Load nonexistent: %v", err)
	}
	if si.Len() != 0 {
		t.Errorf("loading nonexistent file should yield empty index, got %d", si.Len())
	}
}

func TestLoadCorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "search_index.json")

	os.WriteFile(indexPath, []byte("{invalid json"), 0644)

	si := NewSearchIndex(indexPath)
	err := si.Load()
	if err == nil {
		t.Fatal("expected error for corrupted JSON")
	}
}

func TestLoadUnknownVersion(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "search_index.json")

	os.WriteFile(indexPath, []byte(`{"version":99,"entries":{}}`), 0644)

	si := NewSearchIndex(indexPath)
	if err := si.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Should start fresh with an empty index
	if si.Len() != 0 {
		t.Errorf("unknown version should yield empty index, got %d", si.Len())
	}
	if !si.IsDirty() {
		t.Error("unknown version should mark index as dirty for re-save")
	}
}

func TestSaveAtomicity(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "search_index.json")

	si := NewSearchIndex(indexPath)
	si.Upsert(&IndexEntry{Kind: "session", ID: "s1", Title: "test", UpdatedAt: time.Now()})

	if err := si.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// No tmp file should remain
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "search_index.json" {
			t.Errorf("unexpected file left: %s", e.Name())
		}
	}
}

func TestChecksum(t *testing.T) {
	c1 := Checksum([]byte("hello"))
	c2 := Checksum([]byte("hello"))
	c3 := Checksum([]byte("world"))

	if c1 != c2 {
		t.Errorf("same input should produce same checksum: %q != %q", c1, c2)
	}
	if c1 == c3 {
		t.Errorf("different input should produce different checksum: %q == %q", c1, c3)
	}
	if len(c1) != 16 {
		t.Errorf("checksum should be 16 hex chars, got %d", len(c1))
	}
}

func TestChecksumEmpty(t *testing.T) {
	c := Checksum([]byte{})
	if c == "" {
		t.Error("checksum of empty data should not be empty string")
	}
}

func TestIsDirty(t *testing.T) {
	si := NewSearchIndex("")

	if si.IsDirty() {
		t.Error("new index should not be dirty")
	}

	si.Upsert(&IndexEntry{Kind: "session", ID: "s1", Title: "test", UpdatedAt: time.Now()})
	if !si.IsDirty() {
		t.Error("index should be dirty after upsert")
	}
}
