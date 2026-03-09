package search

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMatchExact(t *testing.T) {
	ok, score := Match("GitHub", "GitHub")
	if !ok || score != 1.0 {
		t.Errorf("exact match: ok=%v score=%v, want true 1.0", ok, score)
	}
}

func TestMatchExactCaseInsensitive(t *testing.T) {
	ok, score := Match("github", "GitHub")
	if !ok || score != 1.0 {
		t.Errorf("case-insensitive exact: ok=%v score=%v, want true 1.0", ok, score)
	}
}

func TestMatchPrefix(t *testing.T) {
	ok, score := Match("git", "GitHub")
	if !ok || score != 0.9 {
		t.Errorf("prefix match: ok=%v score=%v, want true 0.9", ok, score)
	}
}

func TestMatchContains(t *testing.T) {
	ok, score := Match("hub", "GitHub")
	if !ok || score != 0.7 {
		t.Errorf("contains match: ok=%v score=%v, want true 0.7", ok, score)
	}
}

func TestMatchNoMatch(t *testing.T) {
	ok, score := Match("gitlab", "GitHub")
	if ok || score != 0 {
		t.Errorf("no match: ok=%v score=%v, want false 0", ok, score)
	}
}

func TestMatchEmptyQuery(t *testing.T) {
	ok, score := Match("", "anything")
	if !ok || score != 1.0 {
		t.Errorf("empty query: ok=%v score=%v, want true 1.0", ok, score)
	}
}

func TestMatchEmptyText(t *testing.T) {
	ok, _ := Match("query", "")
	if ok {
		t.Error("should not match empty text with non-empty query")
	}
}

func TestMatchHostExact(t *testing.T) {
	if !MatchHost("github.com", "https://github.com/user/repo") {
		t.Error("should match exact host")
	}
}

func TestMatchHostPartial(t *testing.T) {
	if !MatchHost("github", "https://github.com/user/repo") {
		t.Error("should match partial host")
	}
}

func TestMatchHostCaseInsensitive(t *testing.T) {
	if !MatchHost("GitHub", "https://github.com/user/repo") {
		t.Error("should match host case-insensitively")
	}
}

func TestMatchHostNoMatch(t *testing.T) {
	if MatchHost("gitlab", "https://github.com/user/repo") {
		t.Error("should not match different host")
	}
}

func TestMatchHostEmpty(t *testing.T) {
	if !MatchHost("", "https://github.com") {
		t.Error("empty host should match anything")
	}
}

func TestMatchHostInvalidURL(t *testing.T) {
	// Should fallback to string search
	if !MatchHost("example", "not-a-url-but-has-example") {
		t.Error("should fallback to string search for invalid URLs")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()

	if !strings.HasPrefix(id1, "ss_") {
		t.Errorf("id should start with ss_, got %q", id1)
	}
	if id1 == id2 {
		t.Error("ids should be unique")
	}
}

func TestSavedSearchSerialization(t *testing.T) {
	ss := SavedSearch{
		ID:   "ss_12345_1",
		Name: "work-repos",
		Query: SearchQuery{
			Query:  "github",
			Mode:   "global",
			Scopes: []string{"tabs", "bookmarks"},
			Tags:   []string{"work"},
			Limit:  50,
		},
		CreatedAt: "2026-03-07T10:00:00Z",
		UpdatedAt: "2026-03-07T10:00:00Z",
	}

	data, err := json.Marshal(ss)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded SavedSearch
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != ss.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, ss.ID)
	}
	if decoded.Name != ss.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, ss.Name)
	}
	if decoded.Query.Query != ss.Query.Query {
		t.Errorf("Query.Query = %q, want %q", decoded.Query.Query, ss.Query.Query)
	}
	if len(decoded.Query.Scopes) != 2 {
		t.Errorf("Query.Scopes len = %d, want 2", len(decoded.Query.Scopes))
	}
	if len(decoded.Query.Tags) != 1 {
		t.Errorf("Query.Tags len = %d, want 1", len(decoded.Query.Tags))
	}
}
