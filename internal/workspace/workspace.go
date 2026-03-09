package workspace

import (
	"fmt"
	"sync/atomic"
	"time"
)

// Workspace represents a named grouping of sessions, collections, bookmarks, and saved searches.
type Workspace struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Description       string   `json:"description,omitempty"`
	Sessions          []string `json:"sessions"`
	Collections       []string `json:"collections"`
	BookmarkFolderIDs []string `json:"bookmarkFolderIds,omitempty"`
	SavedSearchIDs    []string `json:"savedSearchIds,omitempty"`
	Tags              []string `json:"tags,omitempty"`
	Notes             string   `json:"notes,omitempty"`
	Status            string   `json:"status"`                   // "active", "archived"
	DefaultTarget     string   `json:"defaultTarget,omitempty"`
	LastActiveAt      string   `json:"lastActiveAt,omitempty"`
	CreatedAt         string   `json:"createdAt"`
	UpdatedAt         string   `json:"updatedAt"`
}

// WorkspaceSummary is the lightweight representation returned by workspace.list.
type WorkspaceSummary struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	SessionCount    int    `json:"sessionCount"`
	CollectionCount int    `json:"collectionCount"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

// Summary returns a WorkspaceSummary from the full Workspace.
func (w *Workspace) Summary() WorkspaceSummary {
	return WorkspaceSummary{
		ID:              w.ID,
		Name:            w.Name,
		SessionCount:    len(w.Sessions),
		CollectionCount: len(w.Collections),
		CreatedAt:       w.CreatedAt,
		UpdatedAt:       w.UpdatedAt,
	}
}

var wsCounter atomic.Uint64

// GenerateID creates a unique ID with "ws_" prefix for workspaces.
func GenerateID() string {
	n := wsCounter.Add(1)
	return fmt.Sprintf("ws_%d_%d", time.Now().UnixMicro(), n)
}
