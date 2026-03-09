package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ctm/internal/bookmarks"
	"ctm/internal/protocol"
	"ctm/internal/search"
	"ctm/internal/workspace"
)

func (h *Hub) handleSearchAction(ctx context.Context, incoming *incomingMessage) {
	switch incoming.msg.Action {
	case "search.query":
		h.handleSearchQuery(ctx, incoming)
	case "search.saved.list":
		h.handleSearchSavedList(incoming)
	case "search.saved.create":
		h.handleSearchSavedCreate(incoming)
	case "search.saved.delete":
		h.handleSearchSavedDelete(incoming)
	default:
		err := &UnknownActionError{Action: incoming.msg.Action}
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(err), err.Error())
	}
}

func (h *Hub) handleSearchQuery(ctx context.Context, incoming *incomingMessage) {
	var q search.SearchQuery
	if err := json.Unmarshal(incoming.msg.Payload, &q); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid search payload")
		return
	}

	if q.Limit <= 0 {
		q.Limit = 50
	}

	// Determine which scopes to search
	scopeSet := make(map[string]bool)
	if len(q.Scopes) == 0 {
		// Search all scopes
		for _, s := range []string{"tabs", "sessions", "collections", "bookmarks", "workspaces"} {
			scopeSet[s] = true
		}
	} else {
		for _, s := range q.Scopes {
			scopeSet[s] = true
		}
	}

	// Collect results from all scopes in a goroutine to avoid blocking Hub
	go func() {
		var results []search.SearchResult

		// Use the search index for sessions, collections, and workspaces
		// when the index is populated. Fall back to file scanning otherwise.
		indexedKinds := h.searchFromIndex(q, scopeSet)
		results = append(results, indexedKinds...)

		// For scopes not covered by the index, use file scanning
		if scopeSet["sessions"] && !h.indexHasKind("session") {
			results = append(results, h.searchSessions(q)...)
		}
		if scopeSet["collections"] && !h.indexHasKind("collection") {
			results = append(results, h.searchCollections(q)...)
		}
		if scopeSet["workspaces"] && !h.indexHasKind("workspace") {
			results = append(results, h.searchWorkspaces(q)...)
		}
		// Bookmarks and tabs are always searched via their original mechanisms
		if scopeSet["bookmarks"] {
			results = append(results, h.searchBookmarks(q)...)
		}
		if scopeSet["tabs"] {
			tabResults := h.searchTabs(ctx, q, incoming.msg.Target)
			results = append(results, tabResults...)
		}

		// Sort by score descending
		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})

		// Apply limit
		if len(results) > q.Limit {
			results = results[:q.Limit]
		}

		sendResponse(incoming.writer, incoming.msg.ID, map[string]any{
			"results": results,
			"total":   len(results),
		})
	}()
}

func (h *Hub) searchSessions(q search.SearchQuery) []search.SearchResult {
	files, err := listJSONFiles(h.sessionsDir)
	if err != nil {
		return nil
	}

	var results []search.SearchResult
	for _, f := range files {
		var s Session
		if err := loadJSON(filepath.Join(h.sessionsDir, f), &s); err != nil {
			continue
		}

		if ok, score := search.Match(q.Query, s.Name); ok {
			if q.Host == "" || h.sessionContainsHost(s, q.Host) {
				results = append(results, search.SearchResult{
					Kind:       "session",
					ID:         s.Name,
					Title:      s.Name,
					MatchField: "name",
					Score:      score,
				})
			}
		}
	}
	return results
}

func (h *Hub) sessionContainsHost(s Session, host string) bool {
	for _, w := range s.Windows {
		for _, t := range w.Tabs {
			if search.MatchHost(host, t.URL) {
				return true
			}
		}
	}
	return false
}

func (h *Hub) searchCollections(q search.SearchQuery) []search.SearchResult {
	files, err := listJSONFiles(h.collectionsDir)
	if err != nil {
		return nil
	}

	var results []search.SearchResult
	for _, f := range files {
		var c Collection
		if err := loadJSON(filepath.Join(h.collectionsDir, f), &c); err != nil {
			continue
		}

		// Match collection name
		if ok, score := search.Match(q.Query, c.Name); ok {
			results = append(results, search.SearchResult{
				Kind:       "collection",
				ID:         c.Name,
				Title:      c.Name,
				MatchField: "name",
				Score:      score,
			})
			continue
		}

		// Match items within collection
		for _, item := range c.Items {
			matched := false
			var matchField string
			var score float64

			if ok, s := search.Match(q.Query, item.Title); ok {
				matched = true
				matchField = "title"
				score = s
			} else if ok, s := search.Match(q.Query, item.URL); ok {
				matched = true
				matchField = "url"
				score = s
			}

			if matched && (q.Host == "" || search.MatchHost(q.Host, item.URL)) {
				results = append(results, search.SearchResult{
					Kind:       "collection",
					ID:         c.Name,
					Title:      item.Title,
					URL:        item.URL,
					MatchField: matchField,
					Score:      score,
				})
			}
		}
	}
	return results
}

func (h *Hub) searchWorkspaces(q search.SearchQuery) []search.SearchResult {
	files, err := listJSONFiles(h.workspacesDir)
	if err != nil {
		return nil
	}

	var results []search.SearchResult
	for _, f := range files {
		var w workspace.Workspace
		if err := loadJSON(filepath.Join(h.workspacesDir, f), &w); err != nil {
			continue
		}

		if ok, score := search.Match(q.Query, w.Name); ok {
			results = append(results, search.SearchResult{
				Kind:       "workspace",
				ID:         w.ID,
				Title:      w.Name,
				MatchField: "name",
				Score:      score,
			})
			continue
		}

		// Match tags
		for _, tag := range w.Tags {
			if ok, score := search.Match(q.Query, tag); ok {
				results = append(results, search.SearchResult{
					Kind:       "workspace",
					ID:         w.ID,
					Title:      w.Name,
					MatchField: "tag",
					Score:      score,
				})
				break
			}
		}
	}
	return results
}

func (h *Hub) searchBookmarks(q search.SearchQuery) []search.SearchResult {
	// Collect mirrors: try all per-target mirror files, dedup by targetID
	mirrorFiles, _ := listMirrorFiles(h.bookmarksDir)

	var mirrors []bookmarks.BookmarkMirror
	seenTargets := make(map[string]bool)

	// Load per-target mirrors first (they are more specific)
	for _, f := range mirrorFiles {
		if f == "mirror.json" {
			continue // process default last
		}
		var m bookmarks.BookmarkMirror
		if err := loadJSON(filepath.Join(h.bookmarksDir, f), &m); err == nil {
			if m.TargetID != "" && !seenTargets[m.TargetID] {
				seenTargets[m.TargetID] = true
				mirrors = append(mirrors, m)
			}
		}
	}

	// Fall back to default mirror.json if no per-target mirrors found
	if len(mirrors) == 0 {
		var m bookmarks.BookmarkMirror
		if err := loadJSON(filepath.Join(h.bookmarksDir, "mirror.json"), &m); err != nil {
			return nil
		}
		mirrors = append(mirrors, m)
	}

	// Search across all mirrors, dedup by bookmark ID
	seenIDs := make(map[string]bool)
	var results []search.SearchResult
	for _, mirror := range mirrors {
		matches := bookmarks.SearchBookmarks(mirror.Tree, q.Query)
		for _, n := range matches {
			if seenIDs[n.ID] {
				continue
			}
			seenIDs[n.ID] = true

			if q.Host != "" && !search.MatchHost(q.Host, n.URL) {
				continue
			}

			matchField := "title"
			_, titleScore := search.Match(q.Query, n.Title)
			_, urlScore := search.Match(q.Query, n.URL)
			score := titleScore
			if urlScore > titleScore {
				matchField = "url"
				score = urlScore
			}

			results = append(results, search.SearchResult{
				Kind:       "bookmark",
				ID:         n.ID,
				Title:      n.Title,
				URL:        n.URL,
				MatchField: matchField,
				Score:      score,
			})
		}
	}
	return results
}

func (h *Hub) searchTabs(ctx context.Context, q search.SearchQuery, reqTarget *protocol.TargetSelector) []search.SearchResult {
	// Try to get tabs from extension, using the request's target if specified
	target, err := h.resolveTarget(reqTarget)
	if err != nil {
		return nil
	}

	resp, err := h.sendToExtensionAndWait(ctx, target, "tabs.list", nil)
	if err != nil {
		return nil
	}

	var tabsData struct {
		Tabs []struct {
			ID    int    `json:"id"`
			Title string `json:"title"`
			URL   string `json:"url"`
		} `json:"tabs"`
	}
	if json.Unmarshal(resp.Payload, &tabsData) != nil {
		return nil
	}

	var results []search.SearchResult
	for _, tab := range tabsData.Tabs {
		if q.Host != "" && !search.MatchHost(q.Host, tab.URL) {
			continue
		}

		matchField := ""
		var score float64

		if ok, s := search.Match(q.Query, tab.Title); ok {
			matchField = "title"
			score = s
		} else if ok, s := search.Match(q.Query, tab.URL); ok {
			matchField = "url"
			score = s
		}

		if matchField != "" {
			results = append(results, search.SearchResult{
				Kind:       "tab",
				ID:         fmt.Sprintf("%d", tab.ID),
				Title:      tab.Title,
				URL:        tab.URL,
				MatchField: matchField,
				Score:      score,
			})
		}
	}
	return results
}

// --- Search index helpers ---

// searchFromIndex queries the in-memory search index for indexed kinds
// (sessions, collections, workspaces) that match the query.
func (h *Hub) searchFromIndex(q search.SearchQuery, scopeSet map[string]bool) []search.SearchResult {
	// Determine which indexed kinds to query
	var kinds []string
	if scopeSet["sessions"] && h.indexHasKind("session") {
		kinds = append(kinds, "session")
	}
	if scopeSet["collections"] && h.indexHasKind("collection") {
		kinds = append(kinds, "collection")
	}
	if scopeSet["workspaces"] && h.indexHasKind("workspace") {
		kinds = append(kinds, "workspace")
	}

	if len(kinds) == 0 {
		return nil
	}

	entries := h.index.Search(q.Query, kinds)
	var results []search.SearchResult
	for _, entry := range entries {
		// Determine best match field
		matchField := "name"
		bestScore := 0.0

		if ok, s := search.Match(q.Query, entry.Title); ok && s > bestScore {
			matchField = "name"
			bestScore = s
		}
		if ok, s := search.Match(q.Query, entry.URL); ok && s > bestScore {
			matchField = "url"
			bestScore = s
		}
		for _, tag := range entry.Tags {
			if ok, s := search.Match(q.Query, tag); ok && s > bestScore {
				matchField = "tag"
				bestScore = s
			}
		}

		results = append(results, search.SearchResult{
			Kind:       entry.Kind,
			ID:         entry.ID,
			Title:      entry.Title,
			URL:        entry.URL,
			MatchField: matchField,
			Score:      bestScore,
		})
	}
	return results
}

// indexHasKind returns true if the index contains at least one entry of the given kind.
func (h *Hub) indexHasKind(kind string) bool {
	entries := h.index.Search("", []string{kind})
	return len(entries) > 0
}

// --- Saved Search handlers ---

func (h *Hub) handleSearchSavedList(incoming *incomingMessage) {
	files, err := listJSONFiles(h.savedSearchesDir)
	if err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("list saved searches: %s", err))
		return
	}

	searches := make([]search.SavedSearch, 0, len(files))
	for _, f := range files {
		var ss search.SavedSearch
		if err := loadJSON(filepath.Join(h.savedSearchesDir, f), &ss); err != nil {
			continue
		}
		searches = append(searches, ss)
	}

	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{"searches": searches})
}

func (h *Hub) handleSearchSavedCreate(incoming *incomingMessage) {
	var payload struct {
		Name  string             `json:"name"`
		Query search.SearchQuery `json:"query"`
	}
	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid payload")
		return
	}

	if payload.Name == "" {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "name cannot be empty")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	ss := &search.SavedSearch{
		ID:        search.GenerateID(),
		Name:      payload.Name,
		Query:     payload.Query,
		CreatedAt: now,
		UpdatedAt: now,
	}

	filename := ss.ID + ".json"
	if err := atomicWriteJSON(h.savedSearchesDir, filename, ss); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("save failed: %s", err))
		return
	}

	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{
		"id":   ss.ID,
		"name": ss.Name,
	})
}

func (h *Hub) handleSearchSavedDelete(incoming *incomingMessage) {
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid payload")
		return
	}

	if payload.ID == "" || !strings.HasPrefix(payload.ID, "ss_") {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid saved search ID")
		return
	}
	if err := validatePathSafe(payload.ID); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("invalid saved search ID: %s", err))
		return
	}

	path := filepath.Join(h.savedSearchesDir, payload.ID+".json")
	if err := os.Remove(path); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("delete failed: %s", err))
		return
	}

	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{})
}
