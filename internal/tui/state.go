package tui

import "encoding/json"

type ViewType int

const (
	ViewTabs ViewType = iota
	ViewGroups
	ViewSessions
	ViewCollections
	ViewTargets
	// Phase 7+
	ViewBookmarks
	ViewWorkspaces
	ViewSync
	ViewHistory
	ViewSearch
	ViewDownloads
)

func (v ViewType) String() string {
	names := [...]string{"Tabs", "Groups", "Sessions", "Collections", "Targets",
		"Bookmarks", "Workspaces", "Sync", "History", "Search", "Downloads"}
	if int(v) < len(names) {
		return names[v]
	}
	return "Unknown"
}

type InputMode int

const (
	ModeNormal InputMode = iota
	ModeFilter
	ModeCommand
	ModeHelp
	ModeYank
	ModeZFilter
	ModeConfirmDelete
	ModeNameInput
)

type TabItem struct {
	ID       int    `json:"id"`
	WindowID int    `json:"windowId"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Active   bool   `json:"active"`
	Pinned   bool   `json:"pinned"`
	Muted    bool   `json:"muted"`
	GroupID  int    `json:"groupId"`
}

type GroupItem struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Color     string `json:"color"`
	Collapsed bool   `json:"collapsed"`
	WindowID  int    `json:"windowId"`
}

type SessionItem struct {
	Name         string `json:"name"`
	TabCount     int    `json:"tabCount"`
	WindowCount  int    `json:"windowCount"`
	GroupCount   int    `json:"groupCount"`
	CreatedAt    string `json:"createdAt"`
	SourceTarget string `json:"sourceTarget"`
}

type CollectionItem struct {
	Name      string `json:"name"`
	ItemCount int    `json:"itemCount"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type TargetItem struct {
	TargetID    string `json:"targetId"`
	Channel     string `json:"channel"`
	Label       string `json:"label"`
	IsDefault   bool   `json:"isDefault"`
	UserAgent   string `json:"userAgent"`
	ConnectedAt int64  `json:"connectedAt"`
}

type BookmarkItem struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	URL      string         `json:"url,omitempty"`
	ParentID string         `json:"parentId,omitempty"`
	Children []BookmarkItem `json:"children,omitempty"`
	Depth    int            `json:"-"` // rendering depth (not serialized)
	IsFolder bool           `json:"-"` // derived from Children presence
}

// NestedTabItem represents a tab shown inline under an expanded session or collection.
type NestedTabItem struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	Pinned     bool   `json:"pinned"`
	ParentName string `json:"-"` // which session/collection this belongs to
}

type HistoryItem struct {
	ID            string  `json:"id"`
	URL           string  `json:"url"`
	Title         string  `json:"title"`
	LastVisitTime float64 `json:"lastVisitTime"`
	VisitCount    int     `json:"visitCount"`
}

type WorkspaceItem struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	SessionCount    int    `json:"sessionCount"`
	CollectionCount int    `json:"collectionCount"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

type SyncStatusItem struct {
	Enabled        bool     `json:"enabled"`
	SyncDir        string   `json:"syncDir"`
	LastSync       string   `json:"lastSync"`
	PendingChanges int      `json:"pendingChanges"`
	Conflicts      []string `json:"conflicts"`
}

// SearchResultItem represents a result from search.query.
type SearchResultItem struct {
	Kind       string  `json:"kind"`
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	URL        string  `json:"url,omitempty"`
	MatchField string  `json:"matchField"`
	Score      float64 `json:"score"`
}

// SavedSearchItem represents a saved search for TUI display.
type SavedSearchItem struct {
	ID        string
	Name      string
	QueryText string
	CreatedAt string
}

// DateSeparator is a non-interactive header row for grouping items by date.
type DateSeparator struct {
	Label string // e.g. "Today", "Yesterday", "This Week", "2025-03-08"
}

// DownloadItem represents a browser download.
type DownloadItem struct {
	ID         int    `json:"id"`
	Filename   string `json:"filename"`
	URL        string `json:"url"`
	State      string `json:"state"`
	TotalBytes int64  `json:"totalBytes"`
}

type ViewState struct {
	cursor    int
	selected  map[int]bool
	items     []any
	filtered  []int // indices into items matching filter
	itemCount int
}

func newViewState() *ViewState {
	return &ViewState{
		selected: make(map[int]bool),
	}
}

func (vs *ViewState) visibleCount() int {
	if vs.filtered != nil {
		return len(vs.filtered)
	}
	return vs.itemCount
}

func (vs *ViewState) clampCursor() {
	if vs.cursor < 0 {
		vs.cursor = 0
	}
	max := vs.visibleCount() - 1
	if max < 0 {
		max = 0
	}
	if vs.cursor > max {
		vs.cursor = max
	}
}

func (vs *ViewState) realIndex(viewIdx int) int {
	if vs.filtered != nil && viewIdx < len(vs.filtered) {
		return vs.filtered[viewIdx]
	}
	return viewIdx
}

func parsePayload[T any](payload json.RawMessage, key string) []T {
	var data map[string]json.RawMessage
	if json.Unmarshal(payload, &data) != nil {
		return nil
	}
	raw, ok := data[key]
	if !ok {
		return nil
	}
	var items []T
	json.Unmarshal(raw, &items)
	return items
}
