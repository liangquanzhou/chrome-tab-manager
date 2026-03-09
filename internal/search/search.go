package search

import (
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

// SearchQuery defines the parameters for a search operation.
type SearchQuery struct {
	Query  string   `json:"query"`
	Mode   string   `json:"mode"`   // "global"
	Scopes []string `json:"scopes"` // ["tabs", "sessions", "collections", "bookmarks", "workspaces"]
	Tags   []string `json:"tags"`
	Host   string   `json:"host"`
	Limit  int      `json:"limit"`
}

// SearchResult represents a single search match.
type SearchResult struct {
	Kind       string  `json:"kind"`       // "tab", "session", "collection", "bookmark", "workspace"
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	URL        string  `json:"url,omitempty"`
	MatchField string  `json:"matchField"` // "title", "url", "name", "tag"
	Score      float64 `json:"score"`
}

// SavedSearch represents a persisted search query.
type SavedSearch struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Query     SearchQuery `json:"query"`
	CreatedAt string      `json:"createdAt"`
	UpdatedAt string      `json:"updatedAt"`
}

var ssCounter atomic.Uint64

// GenerateID creates a unique ID with "ss_" prefix for saved searches.
func GenerateID() string {
	n := ssCounter.Add(1)
	return fmt.Sprintf("ss_%d_%d", time.Now().UnixMicro(), n)
}

// Match performs fuzzy matching of query against text.
// Returns whether there is a match and a score:
//   - exact match (case-insensitive): score = 1.0
//   - prefix match: score = 0.9
//   - contains match: score = 0.7
//   - no match: false, 0
func Match(query string, text string) (bool, float64) {
	if query == "" {
		return true, 1.0
	}
	q := strings.ToLower(query)
	t := strings.ToLower(text)

	if q == t {
		return true, 1.0
	}
	if strings.HasPrefix(t, q) {
		return true, 0.9
	}
	if strings.Contains(t, q) {
		return true, 0.7
	}
	return false, 0
}

// MatchHost checks if the given host/domain matches the URL's host.
// Supports partial domain matching (e.g., "github" matches "github.com").
func MatchHost(host string, rawURL string) bool {
	if host == "" {
		return true
	}
	h := strings.ToLower(host)

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return strings.Contains(strings.ToLower(rawURL), h)
	}

	urlHost := strings.ToLower(parsed.Hostname())
	if urlHost == "" {
		// No scheme → fallback to string search
		return strings.Contains(strings.ToLower(rawURL), h)
	}

	// Exact match
	if urlHost == h {
		return true
	}
	// Domain contains the host filter
	if strings.Contains(urlHost, h) {
		return true
	}
	return false
}
