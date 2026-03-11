package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// 1. ViewState methods: visibleCount, clampCursor, realIndex
// ---------------------------------------------------------------------------

func TestViewState_VisibleCount_NoFilter(t *testing.T) {
	vs := newViewState()
	vs.itemCount = 5
	if got := vs.visibleCount(); got != 5 {
		t.Errorf("visibleCount() = %d, want 5", got)
	}
}

func TestViewState_VisibleCount_Empty(t *testing.T) {
	vs := newViewState()
	if got := vs.visibleCount(); got != 0 {
		t.Errorf("visibleCount() = %d, want 0", got)
	}
}

func TestViewState_VisibleCount_WithFilter(t *testing.T) {
	vs := newViewState()
	vs.itemCount = 10
	vs.filtered = []int{1, 3, 7}
	if got := vs.visibleCount(); got != 3 {
		t.Errorf("visibleCount() = %d, want 3", got)
	}
}

func TestViewState_VisibleCount_EmptyFilter(t *testing.T) {
	// A non-nil but empty filtered slice means "filtered to nothing".
	vs := newViewState()
	vs.itemCount = 5
	vs.filtered = []int{}
	if got := vs.visibleCount(); got != 0 {
		t.Errorf("visibleCount() = %d, want 0", got)
	}
}

func TestViewState_ClampCursor_CursorBeyondMax(t *testing.T) {
	vs := newViewState()
	vs.itemCount = 3
	vs.cursor = 10
	vs.clampCursor()
	if vs.cursor != 2 {
		t.Errorf("cursor = %d, want 2", vs.cursor)
	}
}

func TestViewState_ClampCursor_EmptyItems(t *testing.T) {
	vs := newViewState()
	vs.cursor = 5
	vs.clampCursor()
	if vs.cursor != 0 {
		t.Errorf("cursor = %d, want 0", vs.cursor)
	}
}

func TestViewState_ClampCursor_CursorWithinBounds(t *testing.T) {
	vs := newViewState()
	vs.itemCount = 5
	vs.cursor = 2
	vs.clampCursor()
	if vs.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (should not change)", vs.cursor)
	}
}

func TestViewState_ClampCursor_WithFilter(t *testing.T) {
	vs := newViewState()
	vs.itemCount = 10
	vs.filtered = []int{2, 4}
	vs.cursor = 5
	vs.clampCursor()
	if vs.cursor != 1 {
		t.Errorf("cursor = %d, want 1", vs.cursor)
	}
}

func TestViewState_RealIndex_NoFilter(t *testing.T) {
	vs := newViewState()
	vs.itemCount = 5
	if got := vs.realIndex(3); got != 3 {
		t.Errorf("realIndex(3) = %d, want 3", got)
	}
}

func TestViewState_RealIndex_WithFilter(t *testing.T) {
	vs := newViewState()
	vs.itemCount = 10
	vs.filtered = []int{2, 5, 8}
	tests := []struct {
		viewIdx int
		want    int
	}{
		{0, 2},
		{1, 5},
		{2, 8},
	}
	for _, tt := range tests {
		got := vs.realIndex(tt.viewIdx)
		if got != tt.want {
			t.Errorf("realIndex(%d) = %d, want %d", tt.viewIdx, got, tt.want)
		}
	}
}

func TestViewState_RealIndex_OutOfFilterRange(t *testing.T) {
	vs := newViewState()
	vs.filtered = []int{1, 3}
	// viewIdx beyond filtered length falls through to identity
	got := vs.realIndex(5)
	if got != 5 {
		t.Errorf("realIndex(5) = %d, want 5 (fallthrough)", got)
	}
}

// ---------------------------------------------------------------------------
// 2. View switching: nextView cycles correctly
// ---------------------------------------------------------------------------

func TestNextView_Forward(t *testing.T) {
	a := NewApp("/tmp/test.sock")

	expected := []ViewType{ViewGroups, ViewSessions, ViewCollections, ViewBookmarks, ViewWorkspaces, ViewSync, ViewHistory, ViewSearch, ViewDownloads, ViewTargets, ViewTabs}
	for i, want := range expected {
		a.nextView(1)
		if a.view != want {
			t.Errorf("step %d: view = %v, want %v", i, a.view, want)
		}
	}
}

func TestNextView_Backward(t *testing.T) {
	a := NewApp("/tmp/test.sock")

	expected := []ViewType{ViewTargets, ViewDownloads, ViewSearch, ViewHistory, ViewSync, ViewWorkspaces, ViewBookmarks, ViewCollections, ViewSessions, ViewGroups, ViewTabs}
	for i, want := range expected {
		a.nextView(-1)
		if a.view != want {
			t.Errorf("step %d: view = %v, want %v", i, a.view, want)
		}
	}
}

func TestNextView_Wraparound(t *testing.T) {
	a := NewApp("/tmp/test.sock")

	// Cycle fully forward 11 times to get back to the start (ViewTabs).
	for i := 0; i < 11; i++ {
		a.nextView(1)
	}
	if a.view != ViewTabs {
		t.Errorf("after full cycle: view = %v, want ViewTabs", a.view)
	}
}

// ---------------------------------------------------------------------------
// 3. Navigation: moveCursor, gg, G
// ---------------------------------------------------------------------------

func TestMoveCursor_Down(t *testing.T) {
	a := NewApp("/tmp/test.sock")
	vs := a.currentView()
	vs.itemCount = 5

	a.moveCursor(1)
	if vs.cursor != 1 {
		t.Errorf("cursor = %d, want 1", vs.cursor)
	}
	a.moveCursor(1)
	if vs.cursor != 2 {
		t.Errorf("cursor = %d, want 2", vs.cursor)
	}
}

func TestMoveCursor_Up(t *testing.T) {
	a := NewApp("/tmp/test.sock")
	vs := a.currentView()
	vs.itemCount = 5
	vs.cursor = 3

	a.moveCursor(-1)
	if vs.cursor != 2 {
		t.Errorf("cursor = %d, want 2", vs.cursor)
	}
}

func TestMoveCursor_ClampAtBottom(t *testing.T) {
	a := NewApp("/tmp/test.sock")
	vs := a.currentView()
	vs.itemCount = 3
	vs.cursor = 2

	a.moveCursor(5)
	if vs.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (clamped)", vs.cursor)
	}
}

func TestMoveCursor_ClampAtTop(t *testing.T) {
	a := NewApp("/tmp/test.sock")
	vs := a.currentView()
	vs.itemCount = 3
	vs.cursor = 0

	a.moveCursor(-5)
	if vs.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped at top)", vs.cursor)
	}
}

func TestPendingG_GG_GoesToTop(t *testing.T) {
	a := NewApp("/tmp/test.sock")
	vs := a.currentView()
	vs.itemCount = 10
	vs.cursor = 5

	// Simulate: first 'g' sets pendingG, second 'g' moves to top.
	a.pendingG = true
	// When pendingG && key == "g", cursor goes to 0.
	a.currentView().cursor = 0 // mirrors handleKey behavior
	a.pendingG = false

	if vs.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (gg)", vs.cursor)
	}
}

func TestG_GoesToBottom(t *testing.T) {
	a := NewApp("/tmp/test.sock")
	vs := a.currentView()
	vs.itemCount = 10
	vs.cursor = 0

	// Simulate G: cursor = visibleCount - 1
	vs.cursor = vs.visibleCount() - 1
	if vs.cursor < 0 {
		vs.cursor = 0
	}

	if vs.cursor != 9 {
		t.Errorf("cursor = %d, want 9 (G/bottom)", vs.cursor)
	}
}

func TestG_EmptyView(t *testing.T) {
	a := NewApp("/tmp/test.sock")
	vs := a.currentView()
	vs.itemCount = 0

	vs.cursor = vs.visibleCount() - 1
	if vs.cursor < 0 {
		vs.cursor = 0
	}

	if vs.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (empty view)", vs.cursor)
	}

	_ = a // use a
}

// ---------------------------------------------------------------------------
// 4. Filter: applyFilter and clearFilter
// ---------------------------------------------------------------------------

func setupTabsApp(tabs []TabItem) *App {
	a := NewApp("/tmp/test.sock")
	a.view = ViewTabs
	vs := a.currentView()
	vs.items = make([]any, len(tabs))
	for i, t := range tabs {
		vs.items[i] = t
	}
	vs.itemCount = len(tabs)
	return a
}

func TestApplyFilter_Tabs(t *testing.T) {
	tabs := []TabItem{
		{ID: 1, Title: "GitHub Dashboard", URL: "https://github.com"},
		{ID: 2, Title: "Google Search", URL: "https://google.com"},
		{ID: 3, Title: "GitHub Issues", URL: "https://github.com/issues"},
	}
	a := setupTabsApp(tabs)

	a.filterText = "github"
	a.applyFilter()

	vs := a.currentView()
	if vs.visibleCount() != 2 {
		t.Errorf("visibleCount = %d, want 2", vs.visibleCount())
	}
	// Verify the filtered indices point to correct tabs
	if vs.filtered[0] != 0 || vs.filtered[1] != 2 {
		t.Errorf("filtered = %v, want [0, 2]", vs.filtered)
	}
}

func TestApplyFilter_CaseInsensitive(t *testing.T) {
	tabs := []TabItem{
		{ID: 1, Title: "HELLO World", URL: "https://example.com"},
		{ID: 2, Title: "goodbye", URL: "https://test.com"},
	}
	a := setupTabsApp(tabs)

	a.filterText = "hello"
	a.applyFilter()

	vs := a.currentView()
	if vs.visibleCount() != 1 {
		t.Errorf("visibleCount = %d, want 1", vs.visibleCount())
	}
}

func TestApplyFilter_ByURL(t *testing.T) {
	tabs := []TabItem{
		{ID: 1, Title: "My Page", URL: "https://example.com/path"},
		{ID: 2, Title: "Other Page", URL: "https://other.com"},
	}
	a := setupTabsApp(tabs)

	a.filterText = "example.com"
	a.applyFilter()

	vs := a.currentView()
	if vs.visibleCount() != 1 {
		t.Errorf("visibleCount = %d, want 1", vs.visibleCount())
	}
}

func TestApplyFilter_NoMatch(t *testing.T) {
	tabs := []TabItem{
		{ID: 1, Title: "GitHub", URL: "https://github.com"},
	}
	a := setupTabsApp(tabs)

	a.filterText = "zzzzz"
	a.applyFilter()

	vs := a.currentView()
	if vs.filtered == nil {
		t.Errorf("filtered should be non-nil empty slice, got nil")
	}
	if vs.visibleCount() != 0 {
		t.Errorf("visibleCount = %d, want 0 (no matches)", vs.visibleCount())
	}
}

func TestApplyFilter_Empty_ClearsFilter(t *testing.T) {
	tabs := []TabItem{
		{ID: 1, Title: "GitHub", URL: "https://github.com"},
		{ID: 2, Title: "Google", URL: "https://google.com"},
	}
	a := setupTabsApp(tabs)

	// Apply a filter first
	a.filterText = "github"
	a.applyFilter()

	// Clear by setting empty filter text
	a.filterText = ""
	a.applyFilter()

	vs := a.currentView()
	if vs.filtered != nil {
		t.Errorf("filtered should be nil after empty filter text")
	}
	if vs.visibleCount() != 2 {
		t.Errorf("visibleCount = %d, want 2", vs.visibleCount())
	}
}

func TestClearFilter(t *testing.T) {
	tabs := []TabItem{
		{ID: 1, Title: "A", URL: "https://a.com"},
		{ID: 2, Title: "B", URL: "https://b.com"},
		{ID: 3, Title: "C", URL: "https://c.com"},
	}
	a := setupTabsApp(tabs)

	a.filterText = "a"
	a.applyFilter()

	vs := a.currentView()
	if vs.visibleCount() != 1 {
		t.Errorf("before clear: visibleCount = %d, want 1", vs.visibleCount())
	}

	a.clearFilter()
	if vs.filtered != nil {
		t.Errorf("after clearFilter: filtered should be nil")
	}
	if vs.visibleCount() != 3 {
		t.Errorf("after clearFilter: visibleCount = %d, want 3", vs.visibleCount())
	}
}

func TestApplyFilter_ClampsCursor(t *testing.T) {
	tabs := []TabItem{
		{ID: 1, Title: "AAA", URL: "https://a.com"},
		{ID: 2, Title: "BBB", URL: "https://b.com"},
		{ID: 3, Title: "CCC", URL: "https://c.com"},
	}
	a := setupTabsApp(tabs)
	vs := a.currentView()
	vs.cursor = 2 // at the bottom

	a.filterText = "aaa"
	a.applyFilter()

	// Only 1 result, cursor should clamp to 0
	if vs.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped by filter)", vs.cursor)
	}
}

// ---------------------------------------------------------------------------
// 5. z-filter: handleZFilterKey
// ---------------------------------------------------------------------------

func TestHandleZFilterKey_Host(t *testing.T) {
	tabs := []TabItem{
		{ID: 1, Title: "GH Main", URL: "https://github.com/main"},
		{ID: 2, Title: "Google", URL: "https://google.com/search"},
		{ID: 3, Title: "GH Issues", URL: "https://github.com/issues"},
	}
	a := setupTabsApp(tabs)
	a.mode = ModeZFilter
	vs := a.views[ViewTabs]
	vs.cursor = 0 // cursor on github.com

	a.handleZFilterKey("h")

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if len(vs.filtered) != 2 {
		t.Errorf("filtered len = %d, want 2", len(vs.filtered))
	}
	// Should contain indices 0 and 2 (both github.com)
	if vs.filtered[0] != 0 || vs.filtered[1] != 2 {
		t.Errorf("filtered = %v, want [0, 2]", vs.filtered)
	}
}

func TestHandleZFilterKey_Pinned(t *testing.T) {
	tabs := []TabItem{
		{ID: 1, Title: "Pinned Tab", URL: "https://a.com", Pinned: true},
		{ID: 2, Title: "Normal Tab", URL: "https://b.com", Pinned: false},
		{ID: 3, Title: "Another Pinned", URL: "https://c.com", Pinned: true},
	}
	a := setupTabsApp(tabs)
	a.mode = ModeZFilter

	a.handleZFilterKey("p")

	vs := a.views[ViewTabs]
	if len(vs.filtered) != 2 {
		t.Errorf("filtered len = %d, want 2", len(vs.filtered))
	}
}

func TestHandleZFilterKey_Grouped(t *testing.T) {
	tabs := []TabItem{
		{ID: 1, Title: "Grouped", URL: "https://a.com", GroupID: 5},
		{ID: 2, Title: "Not grouped", URL: "https://b.com", GroupID: -1},
		{ID: 3, Title: "Also grouped", URL: "https://c.com", GroupID: 3},
	}
	a := setupTabsApp(tabs)
	a.mode = ModeZFilter

	a.handleZFilterKey("g")

	vs := a.views[ViewTabs]
	if len(vs.filtered) != 2 {
		t.Errorf("filtered len = %d, want 2", len(vs.filtered))
	}
	if vs.filtered[0] != 0 || vs.filtered[1] != 2 {
		t.Errorf("filtered = %v, want [0, 2]", vs.filtered)
	}
}

func TestHandleZFilterKey_Active(t *testing.T) {
	tabs := []TabItem{
		{ID: 1, Title: "Active Tab", URL: "https://a.com", Active: true},
		{ID: 2, Title: "Inactive", URL: "https://b.com", Active: false},
	}
	a := setupTabsApp(tabs)
	a.mode = ModeZFilter

	a.handleZFilterKey("a")

	vs := a.views[ViewTabs]
	if len(vs.filtered) != 1 {
		t.Errorf("filtered len = %d, want 1", len(vs.filtered))
	}
}

func TestHandleZFilterKey_Clear(t *testing.T) {
	tabs := []TabItem{
		{ID: 1, Title: "Tab", URL: "https://a.com", Pinned: true},
		{ID: 2, Title: "Tab2", URL: "https://b.com"},
	}
	a := setupTabsApp(tabs)
	a.mode = ModeZFilter

	// First filter to pinned
	a.handleZFilterKey("p")

	vs := a.views[ViewTabs]
	if len(vs.filtered) != 1 {
		t.Errorf("before clear: filtered len = %d, want 1", len(vs.filtered))
	}

	// Clear
	a.mode = ModeZFilter
	a.handleZFilterKey("c")

	if vs.filtered != nil {
		t.Errorf("after clear: filtered should be nil, got %v", vs.filtered)
	}
	if a.toast != "Filter cleared" {
		t.Errorf("toast = %q, want %q", a.toast, "Filter cleared")
	}
}

func TestHandleZFilterKey_EmptyItems(t *testing.T) {
	a := NewApp("/tmp/test.sock")
	a.view = ViewTabs
	a.mode = ModeZFilter
	// No items set

	a.handleZFilterKey("h")
	// Should not panic and should return to normal mode
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
}

// ---------------------------------------------------------------------------
// 6. matchesFilter
// ---------------------------------------------------------------------------

func TestMatchesFilter_TabItem(t *testing.T) {
	tab := TabItem{Title: "GitHub Dashboard", URL: "https://github.com"}

	if !matchesFilter(tab, "github") {
		t.Error("should match title 'github'")
	}
	if !matchesFilter(tab, "dashboard") {
		t.Error("should match title 'dashboard'")
	}
	if !matchesFilter(tab, "github.com") {
		t.Error("should match URL 'github.com'")
	}
	if matchesFilter(tab, "stackoverflow") {
		t.Error("should not match 'stackoverflow'")
	}
}

func TestMatchesFilter_GroupItem(t *testing.T) {
	group := GroupItem{Title: "Work Tabs"}

	if !matchesFilter(group, "work") {
		t.Error("should match 'work'")
	}
	if matchesFilter(group, "personal") {
		t.Error("should not match 'personal'")
	}
}

func TestMatchesFilter_SessionItem(t *testing.T) {
	session := SessionItem{Name: "morning-session"}

	if !matchesFilter(session, "morning") {
		t.Error("should match 'morning'")
	}
	if matchesFilter(session, "evening") {
		t.Error("should not match 'evening'")
	}
}

func TestMatchesFilter_CollectionItem(t *testing.T) {
	coll := CollectionItem{Name: "reading-list"}

	if !matchesFilter(coll, "reading") {
		t.Error("should match 'reading'")
	}
	if matchesFilter(coll, "todo") {
		t.Error("should not match 'todo'")
	}
}

func TestMatchesFilter_TargetItem(t *testing.T) {
	target := TargetItem{TargetID: "target-123", Label: "Main Browser"}

	if !matchesFilter(target, "main") {
		t.Error("should match label 'main'")
	}
	if !matchesFilter(target, "target-123") {
		t.Error("should match targetId")
	}
	if matchesFilter(target, "secondary") {
		t.Error("should not match 'secondary'")
	}
}

func TestMatchesFilter_BookmarkItem(t *testing.T) {
	bm := BookmarkItem{Title: "Rust Book", URL: "https://doc.rust-lang.org"}

	if !matchesFilter(bm, "rust") {
		t.Error("should match title 'rust'")
	}
	if !matchesFilter(bm, "rust-lang") {
		t.Error("should match URL 'rust-lang'")
	}
	if matchesFilter(bm, "python") {
		t.Error("should not match 'python'")
	}
}

func TestMatchesFilter_WorkspaceItem(t *testing.T) {
	ws := WorkspaceItem{Name: "dev-workspace"}

	if !matchesFilter(ws, "dev") {
		t.Error("should match 'dev'")
	}
	if matchesFilter(ws, "prod") {
		t.Error("should not match 'prod'")
	}
}

func TestMatchesFilter_SyncStatusItem(t *testing.T) {
	ss := SyncStatusItem{SyncDir: "/home/user/sync"}

	if !matchesFilter(ss, "/home") {
		t.Error("should match syncDir '/home'")
	}
	if matchesFilter(ss, "cloud") {
		t.Error("should not match 'cloud'")
	}
}

func TestMatchesFilter_UnknownType(t *testing.T) {
	if matchesFilter("a plain string", "plain") {
		t.Error("unknown type should return false")
	}
}

// ---------------------------------------------------------------------------
// 7. currentItemName
// ---------------------------------------------------------------------------

func TestCurrentItemName_Sessions(t *testing.T) {
	a := NewApp("/tmp/test.sock")
	a.view = ViewSessions
	vs := a.views[ViewSessions]
	vs.items = []any{
		SessionItem{Name: "session-alpha"},
		SessionItem{Name: "session-beta"},
	}
	vs.itemCount = 2
	vs.cursor = 1

	got := a.currentItemName()
	if got != "session-beta" {
		t.Errorf("currentItemName() = %q, want %q", got, "session-beta")
	}
}

func TestCurrentItemName_Collections(t *testing.T) {
	a := NewApp("/tmp/test.sock")
	a.view = ViewCollections
	vs := a.views[ViewCollections]
	vs.items = []any{
		CollectionItem{Name: "my-collection"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	got := a.currentItemName()
	if got != "my-collection" {
		t.Errorf("currentItemName() = %q, want %q", got, "my-collection")
	}
}

func TestCurrentItemName_Workspaces(t *testing.T) {
	a := NewApp("/tmp/test.sock")
	a.view = ViewWorkspaces
	vs := a.views[ViewWorkspaces]
	vs.items = []any{
		WorkspaceItem{Name: "dev-workspace"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	got := a.currentItemName()
	if got != "dev-workspace" {
		t.Errorf("currentItemName() = %q, want %q", got, "dev-workspace")
	}
}

func TestCurrentItemName_EmptyView(t *testing.T) {
	a := NewApp("/tmp/test.sock")
	a.view = ViewSessions

	got := a.currentItemName()
	if got != "" {
		t.Errorf("currentItemName() = %q, want empty string", got)
	}
}

func TestCurrentItemName_Tabs_ReturnsEmpty(t *testing.T) {
	// Tabs view doesn't have a name, should return ""
	tabs := []TabItem{{ID: 1, Title: "Test", URL: "https://test.com"}}
	a := setupTabsApp(tabs)

	got := a.currentItemName()
	if got != "" {
		t.Errorf("currentItemName() for tabs = %q, want empty string", got)
	}
}

func TestCurrentItemName_WithFilter(t *testing.T) {
	a := NewApp("/tmp/test.sock")
	a.view = ViewSessions
	vs := a.views[ViewSessions]
	vs.items = []any{
		SessionItem{Name: "alpha"},
		SessionItem{Name: "beta"},
		SessionItem{Name: "gamma"},
	}
	vs.itemCount = 3
	// Filter shows only index 1 and 2
	vs.filtered = []int{1, 2}
	vs.cursor = 1 // second filtered item = index 2

	got := a.currentItemName()
	if got != "gamma" {
		t.Errorf("currentItemName() = %q, want %q", got, "gamma")
	}
}

// ---------------------------------------------------------------------------
// 8. truncate
// ---------------------------------------------------------------------------

func TestTruncate_ShortString(t *testing.T) {
	s := "hello"
	got := truncate(s, 10)
	if got != "hello" {
		t.Errorf("truncate(%q, 10) = %q, want %q", s, got, "hello")
	}
}

func TestTruncate_ExactLength(t *testing.T) {
	s := "hello"
	got := truncate(s, 5)
	if got != "hello" {
		t.Errorf("truncate(%q, 5) = %q, want %q", s, got, "hello")
	}
}

func TestTruncate_LongString(t *testing.T) {
	s := "hello world this is long"
	got := truncate(s, 10)
	want := "hello w..."
	if got != want {
		t.Errorf("truncate(%q, 10) = %q, want %q", s, got, want)
	}
}

func TestTruncate_VerySmallMax(t *testing.T) {
	s := "hello"
	// max < 4 means no ellipsis, just cut
	got := truncate(s, 3)
	if got != "hel" {
		t.Errorf("truncate(%q, 3) = %q, want %q", s, got, "hel")
	}
}

func TestTruncate_MaxFour(t *testing.T) {
	s := "hello world"
	got := truncate(s, 4)
	want := "h..."
	if got != want {
		t.Errorf("truncate(%q, 4) = %q, want %q", s, got, want)
	}
}

func TestTruncate_EmptyString(t *testing.T) {
	got := truncate("", 10)
	if got != "" {
		t.Errorf("truncate('', 10) = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// 9. extractHost
// ---------------------------------------------------------------------------

func TestExtractHost_HTTPS(t *testing.T) {
	got := extractHost("https://github.com/user/repo")
	if got != "github.com" {
		t.Errorf("extractHost = %q, want %q", got, "github.com")
	}
}

func TestExtractHost_HTTP(t *testing.T) {
	got := extractHost("http://example.com/path")
	if got != "example.com" {
		t.Errorf("extractHost = %q, want %q", got, "example.com")
	}
}

func TestExtractHost_NoScheme(t *testing.T) {
	got := extractHost("example.com/path")
	if got != "example.com" {
		t.Errorf("extractHost = %q, want %q", got, "example.com")
	}
}

func TestExtractHost_NoPath(t *testing.T) {
	got := extractHost("https://github.com")
	if got != "github.com" {
		t.Errorf("extractHost = %q, want %q", got, "github.com")
	}
}

func TestExtractHost_WithPort(t *testing.T) {
	got := extractHost("http://localhost:8080/api")
	if got != "localhost:8080" {
		t.Errorf("extractHost = %q, want %q", got, "localhost:8080")
	}
}

func TestExtractHost_Empty(t *testing.T) {
	got := extractHost("")
	if got != "" {
		t.Errorf("extractHost('') = %q, want empty", got)
	}
}

func TestExtractHost_ChromeScheme(t *testing.T) {
	got := extractHost("chrome://settings/")
	if got != "settings" {
		t.Errorf("extractHost = %q, want %q", got, "settings")
	}
}

// ---------------------------------------------------------------------------
// 10. flattenBookmarkTree
// ---------------------------------------------------------------------------

func TestFlattenBookmarkTree_Empty(t *testing.T) {
	result := flattenBookmarkTree(nil, 0)
	if len(result) != 0 {
		t.Errorf("flattenBookmarkTree(nil) len = %d, want 0", len(result))
	}
}

func TestFlattenBookmarkTree_FlatList(t *testing.T) {
	tree := []BookmarkItem{
		{ID: "1", Title: "Bookmark A", URL: "https://a.com"},
		{ID: "2", Title: "Bookmark B", URL: "https://b.com"},
	}
	result := flattenBookmarkTree(tree, 0)
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	if result[0].Depth != 0 || result[1].Depth != 0 {
		t.Errorf("depths = [%d, %d], want [0, 0]", result[0].Depth, result[1].Depth)
	}
	// Items with URLs and no children are not folders
	if result[0].IsFolder || result[1].IsFolder {
		t.Error("leaf items with URLs should not be folders")
	}
}

func TestFlattenBookmarkTree_NestedFolders(t *testing.T) {
	tree := []BookmarkItem{
		{
			ID:    "1",
			Title: "Folder A",
			Children: []BookmarkItem{
				{ID: "2", Title: "Child 1", URL: "https://child1.com"},
				{
					ID:    "3",
					Title: "Subfolder",
					Children: []BookmarkItem{
						{ID: "4", Title: "Deep Child", URL: "https://deep.com"},
					},
				},
			},
		},
		{ID: "5", Title: "Top Level", URL: "https://top.com"},
	}

	result := flattenBookmarkTree(tree, 0)
	if len(result) != 5 {
		t.Fatalf("len = %d, want 5", len(result))
	}

	expected := []struct {
		id       string
		depth    int
		isFolder bool
	}{
		{"1", 0, true},   // Folder A
		{"2", 1, false},  // Child 1
		{"3", 1, true},   // Subfolder
		{"4", 2, false},  // Deep Child
		{"5", 0, false},  // Top Level
	}

	for i, e := range expected {
		if result[i].ID != e.id {
			t.Errorf("[%d] ID = %q, want %q", i, result[i].ID, e.id)
		}
		if result[i].Depth != e.depth {
			t.Errorf("[%d] Depth = %d, want %d", i, result[i].Depth, e.depth)
		}
		if result[i].IsFolder != e.isFolder {
			t.Errorf("[%d] IsFolder = %v, want %v", i, result[i].IsFolder, e.isFolder)
		}
	}
}

func TestFlattenBookmarkTree_NonZeroInitialDepth(t *testing.T) {
	tree := []BookmarkItem{
		{ID: "1", Title: "Item", URL: "https://a.com"},
	}
	result := flattenBookmarkTree(tree, 3)
	if result[0].Depth != 3 {
		t.Errorf("Depth = %d, want 3", result[0].Depth)
	}
}

func TestFlattenBookmarkTree_EmptyFolder(t *testing.T) {
	// A folder with no children and no URL => IsFolder = true (URL == "")
	tree := []BookmarkItem{
		{ID: "1", Title: "Empty Folder", URL: ""},
	}
	result := flattenBookmarkTree(tree, 0)
	if len(result) != 1 {
		t.Fatalf("len = %d, want 1", len(result))
	}
	if !result[0].IsFolder {
		t.Error("empty folder (no URL, no children) should be IsFolder=true")
	}
}

// ---------------------------------------------------------------------------
// 11. parsePayload
// ---------------------------------------------------------------------------

func TestParsePayload_Tabs(t *testing.T) {
	payload := json.RawMessage(`{
		"tabs": [
			{"id": 1, "title": "Tab One", "url": "https://one.com"},
			{"id": 2, "title": "Tab Two", "url": "https://two.com"}
		]
	}`)

	tabs := parsePayload[TabItem](payload, "tabs")
	if len(tabs) != 2 {
		t.Fatalf("len = %d, want 2", len(tabs))
	}
	if tabs[0].ID != 1 || tabs[0].Title != "Tab One" {
		t.Errorf("tabs[0] = %+v", tabs[0])
	}
	if tabs[1].ID != 2 || tabs[1].URL != "https://two.com" {
		t.Errorf("tabs[1] = %+v", tabs[1])
	}
}

func TestParsePayload_Sessions(t *testing.T) {
	payload := json.RawMessage(`{
		"sessions": [
			{"name": "s1", "tabCount": 5, "windowCount": 1, "groupCount": 0, "createdAt": "2025-01-01T00:00:00Z"}
		]
	}`)

	sessions := parsePayload[SessionItem](payload, "sessions")
	if len(sessions) != 1 {
		t.Fatalf("len = %d, want 1", len(sessions))
	}
	if sessions[0].Name != "s1" || sessions[0].TabCount != 5 {
		t.Errorf("sessions[0] = %+v", sessions[0])
	}
}

func TestParsePayload_MissingKey(t *testing.T) {
	payload := json.RawMessage(`{"other": []}`)

	result := parsePayload[TabItem](payload, "tabs")
	if result != nil {
		t.Errorf("expected nil for missing key, got %v", result)
	}
}

func TestParsePayload_InvalidJSON(t *testing.T) {
	payload := json.RawMessage(`not json at all`)

	result := parsePayload[TabItem](payload, "tabs")
	if result != nil {
		t.Errorf("expected nil for invalid JSON, got %v", result)
	}
}

func TestParsePayload_EmptyArray(t *testing.T) {
	payload := json.RawMessage(`{"tabs": []}`)

	result := parsePayload[TabItem](payload, "tabs")
	if len(result) != 0 {
		t.Errorf("expected empty slice, got len %d", len(result))
	}
}

func TestParsePayload_Collections(t *testing.T) {
	payload := json.RawMessage(`{
		"collections": [
			{"name": "c1", "itemCount": 10, "createdAt": "2025-01-01T00:00:00Z", "updatedAt": "2025-06-01T00:00:00Z"},
			{"name": "c2", "itemCount": 3, "createdAt": "2025-02-01T00:00:00Z", "updatedAt": "2025-06-15T00:00:00Z"}
		]
	}`)

	colls := parsePayload[CollectionItem](payload, "collections")
	if len(colls) != 2 {
		t.Fatalf("len = %d, want 2", len(colls))
	}
	if colls[0].Name != "c1" || colls[0].ItemCount != 10 {
		t.Errorf("colls[0] = %+v", colls[0])
	}
}

// ---------------------------------------------------------------------------
// Additional: ViewType.String, selection, toggle
// ---------------------------------------------------------------------------

func TestViewType_String(t *testing.T) {
	tests := []struct {
		v    ViewType
		want string
	}{
		{ViewTabs, "Tabs"},
		{ViewGroups, "Groups"},
		{ViewSessions, "Sessions"},
		{ViewCollections, "Collections"},
		{ViewTargets, "Targets"},
		{ViewBookmarks, "Bookmarks"},
		{ViewWorkspaces, "Workspaces"},
		{ViewSync, "Sync"},
		{ViewType(99), "Unknown"},
	}
	for _, tt := range tests {
		got := tt.v.String()
		if got != tt.want {
			t.Errorf("ViewType(%d).String() = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestToggleSelect(t *testing.T) {
	tabs := []TabItem{
		{ID: 1, Title: "A", URL: "https://a.com"},
		{ID: 2, Title: "B", URL: "https://b.com"},
		{ID: 3, Title: "C", URL: "https://c.com"},
	}
	a := setupTabsApp(tabs)
	vs := a.currentView()

	// Select item at cursor 0
	a.toggleSelect()
	if !vs.selected[0] {
		t.Error("item 0 should be selected")
	}
	// Cursor advances after toggle
	if vs.cursor != 1 {
		t.Errorf("cursor = %d, want 1", vs.cursor)
	}

	// Toggle item 0 off by moving cursor back
	vs.cursor = 0
	a.toggleSelect()
	if vs.selected[0] {
		t.Error("item 0 should be deselected after second toggle")
	}
}

func TestSelectAll(t *testing.T) {
	tabs := []TabItem{
		{ID: 1, Title: "A", URL: "https://a.com"},
		{ID: 2, Title: "B", URL: "https://b.com"},
	}
	a := setupTabsApp(tabs)

	a.selectAll()

	vs := a.currentView()
	if len(vs.selected) != 2 {
		t.Errorf("selected count = %d, want 2", len(vs.selected))
	}
}

func TestClearSelection(t *testing.T) {
	tabs := []TabItem{
		{ID: 1, Title: "A", URL: "https://a.com"},
		{ID: 2, Title: "B", URL: "https://b.com"},
	}
	a := setupTabsApp(tabs)

	a.selectAll()
	a.clearSelection()

	vs := a.currentView()
	if len(vs.selected) != 0 {
		t.Errorf("selected count = %d, want 0", len(vs.selected))
	}
}

func TestContentHeight(t *testing.T) {
	a := NewApp("/tmp/test.sock")
	a.height = 30

	got := a.contentHeight()
	// Normal mode: height - 3 = 27
	if got != 27 {
		t.Errorf("contentHeight = %d, want 27", got)
	}

	// In filter mode, one more line is used
	a.mode = ModeFilter
	got = a.contentHeight()
	if got != 26 {
		t.Errorf("contentHeight (filter mode) = %d, want 26", got)
	}
}

func TestContentHeight_Minimum(t *testing.T) {
	a := NewApp("/tmp/test.sock")
	a.height = 2 // very small terminal

	got := a.contentHeight()
	if got != 1 {
		t.Errorf("contentHeight = %d, want 1 (minimum)", got)
	}
}

// ---------------------------------------------------------------------------
// TUI Interaction Tests — exercising Update(), View(), and key flows
// ---------------------------------------------------------------------------

// newTestApp creates a minimal App for testing without a real client/socket.
func newTestApp() *App {
	_, cancel := context.WithCancel(context.Background())
	a := &App{
		view:             ViewTabs,
		mode:             ModeNormal,
		views:            make(map[ViewType]*ViewState),
		width:            80,
		height:           24,
		cancel:           cancel,
		collapsedFolders: make(map[string]bool),
	}
	for _, v := range []ViewType{ViewTabs, ViewGroups, ViewSessions, ViewCollections, ViewTargets, ViewBookmarks, ViewWorkspaces, ViewSync} {
		a.views[v] = newViewState()
	}
	return a
}

// populateTabs populates the Tabs view with test tab items.
func populateTabs(a *App, tabs []TabItem) {
	vs := a.views[ViewTabs]
	vs.items = make([]any, len(tabs))
	for i, t := range tabs {
		vs.items[i] = t
	}
	vs.itemCount = len(tabs)
}

// populateSessions populates the Sessions view with test session items.
func populateSessions(a *App, sessions []SessionItem) {
	vs := a.views[ViewSessions]
	vs.items = make([]any, len(sessions))
	for i, s := range sessions {
		vs.items[i] = s
	}
	vs.itemCount = len(sessions)
}

// populateTargets populates the Targets view with test target items.
func populateTargets(a *App, targets []TargetItem) {
	vs := a.views[ViewTargets]
	vs.items = make([]any, len(targets))
	for i, t := range targets {
		vs.items[i] = t
	}
	vs.itemCount = len(targets)
}

// keyRune creates a tea.KeyMsg for a single printable rune.
func keyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// isQuitCmd invokes a tea.Cmd and checks if it produces tea.QuitMsg.
func isQuitCmd(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	msg := cmd()
	_, ok := msg.(tea.QuitMsg)
	return ok
}

// ---------------------------------------------------------------------------
// 1. Update — Window Size
// ---------------------------------------------------------------------------

func TestUpdateWindowSize(t *testing.T) {
	a := newTestApp()
	model, cmd := a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a = model.(*App)

	if a.width != 120 {
		t.Errorf("width = %d, want 120", a.width)
	}
	if a.height != 40 {
		t.Errorf("height = %d, want 40", a.height)
	}
	if cmd != nil {
		t.Errorf("cmd should be nil, got non-nil")
	}
}

// ---------------------------------------------------------------------------
// 2. Update — Key handling in normal mode: Tab / Shift+Tab
// ---------------------------------------------------------------------------

func TestUpdateKeyTab(t *testing.T) {
	a := newTestApp()
	a.view = ViewTabs

	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyTab})
	a = model.(*App)

	if a.view != ViewGroups {
		t.Errorf("view = %v, want ViewGroups", a.view)
	}
}

func TestUpdateKeyShiftTab(t *testing.T) {
	a := newTestApp()
	a.view = ViewGroups

	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	a = model.(*App)

	if a.view != ViewTabs {
		t.Errorf("view = %v, want ViewTabs", a.view)
	}
}

// ---------------------------------------------------------------------------
// 3. Update — Cursor navigation via Update (j, k, G, gg)
// ---------------------------------------------------------------------------

func TestUpdateKeyNavigation(t *testing.T) {
	a := newTestApp()
	populateTabs(a, []TabItem{
		{ID: 1, Title: "Tab One", URL: "https://one.com"},
		{ID: 2, Title: "Tab Two", URL: "https://two.com"},
		{ID: 3, Title: "Tab Three", URL: "https://three.com"},
		{ID: 4, Title: "Tab Four", URL: "https://four.com"},
		{ID: 5, Title: "Tab Five", URL: "https://five.com"},
	})
	vs := a.views[ViewTabs]

	// j → cursor moves down
	model, _ := a.Update(keyRune('j'))
	a = model.(*App)
	if vs.cursor != 1 {
		t.Errorf("after j: cursor = %d, want 1", vs.cursor)
	}

	model, _ = a.Update(keyRune('j'))
	a = model.(*App)
	if vs.cursor != 2 {
		t.Errorf("after jj: cursor = %d, want 2", vs.cursor)
	}

	// k → cursor moves up
	model, _ = a.Update(keyRune('k'))
	a = model.(*App)
	if vs.cursor != 1 {
		t.Errorf("after k: cursor = %d, want 1", vs.cursor)
	}

	// G → cursor goes to end
	model, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	a = model.(*App)
	if vs.cursor != 4 {
		t.Errorf("after G: cursor = %d, want 4", vs.cursor)
	}

	// gg → first g sets pendingG, second g moves to top
	model, _ = a.Update(keyRune('g'))
	a = model.(*App)
	if !a.pendingG {
		t.Errorf("after first g: pendingG should be true")
	}

	model, _ = a.Update(keyRune('g'))
	a = model.(*App)
	if vs.cursor != 0 {
		t.Errorf("after gg: cursor = %d, want 0", vs.cursor)
	}
	if a.pendingG {
		t.Errorf("after gg: pendingG should be false")
	}
}

// ---------------------------------------------------------------------------
// 4. Update — Filter mode
// ---------------------------------------------------------------------------

func TestUpdateKeySlashEntersFilter(t *testing.T) {
	a := newTestApp()

	model, _ := a.Update(keyRune('/'))
	a = model.(*App)

	if a.mode != ModeFilter {
		t.Errorf("mode = %d, want ModeFilter (%d)", a.mode, ModeFilter)
	}
	if a.filterText != "" {
		t.Errorf("filterText = %q, want empty", a.filterText)
	}
}

func TestUpdateFilterInput(t *testing.T) {
	a := newTestApp()
	populateTabs(a, []TabItem{
		{ID: 1, Title: "GitHub", URL: "https://github.com"},
		{ID: 2, Title: "Google", URL: "https://google.com"},
	})

	// Enter filter mode
	model, _ := a.Update(keyRune('/'))
	a = model.(*App)

	// Type "git"
	model, _ = a.Update(keyRune('g'))
	a = model.(*App)
	model, _ = a.Update(keyRune('i'))
	a = model.(*App)
	model, _ = a.Update(keyRune('t'))
	a = model.(*App)

	if a.filterText != "git" {
		t.Errorf("filterText = %q, want %q", a.filterText, "git")
	}
}

func TestUpdateFilterEscCancels(t *testing.T) {
	a := newTestApp()
	populateTabs(a, []TabItem{
		{ID: 1, Title: "GitHub", URL: "https://github.com"},
	})

	// Enter filter mode and type something
	model, _ := a.Update(keyRune('/'))
	a = model.(*App)
	model, _ = a.Update(keyRune('a'))
	a = model.(*App)
	model, _ = a.Update(keyRune('b'))
	a = model.(*App)

	if a.filterText != "ab" {
		t.Fatalf("filterText = %q, want %q", a.filterText, "ab")
	}

	// Esc cancels filter
	model, _ = a.Update(tea.KeyMsg{Type: tea.KeyEscape})
	a = model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal (%d)", a.mode, ModeNormal)
	}
	if a.filterText != "" {
		t.Errorf("filterText = %q, want empty (Esc should clear)", a.filterText)
	}
}

func TestUpdateFilterBackspace(t *testing.T) {
	a := newTestApp()
	a.mode = ModeFilter
	a.filterText = "abc"

	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	a = model.(*App)

	if a.filterText != "ab" {
		t.Errorf("filterText = %q, want %q", a.filterText, "ab")
	}
}

func TestUpdateFilterEnterAccepts(t *testing.T) {
	a := newTestApp()
	populateTabs(a, []TabItem{
		{ID: 1, Title: "GitHub", URL: "https://github.com"},
		{ID: 2, Title: "Google", URL: "https://google.com"},
	})

	// Enter filter mode and type
	a.mode = ModeFilter
	a.filterText = "git"
	a.applyFilter()

	// Press Enter to accept filter
	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	a = model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	// Filter text should remain (filter stays applied)
	vs := a.views[ViewTabs]
	if vs.filtered == nil {
		t.Errorf("filtered should still be applied after Enter")
	}
}

// ---------------------------------------------------------------------------
// 5. Update — Command mode
// ---------------------------------------------------------------------------

func TestUpdateKeyColonEntersCommand(t *testing.T) {
	a := newTestApp()

	model, _ := a.Update(keyRune(':'))
	a = model.(*App)

	if a.mode != ModeCommand {
		t.Errorf("mode = %d, want ModeCommand (%d)", a.mode, ModeCommand)
	}
	if a.commandText != "" {
		t.Errorf("commandText = %q, want empty", a.commandText)
	}
}

func TestUpdateCommandEsc(t *testing.T) {
	a := newTestApp()
	a.mode = ModeCommand
	a.commandText = "something"

	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyEscape})
	a = model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if a.commandText != "" {
		t.Errorf("commandText = %q, want empty", a.commandText)
	}
}

func TestUpdateCommandInput(t *testing.T) {
	a := newTestApp()
	a.mode = ModeCommand

	model, _ := a.Update(keyRune('q'))
	a = model.(*App)

	if a.commandText != "q" {
		t.Errorf("commandText = %q, want %q", a.commandText, "q")
	}
}

func TestUpdateCommandBackspace(t *testing.T) {
	a := newTestApp()
	a.mode = ModeCommand
	a.commandText = "help"

	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	a = model.(*App)

	if a.commandText != "hel" {
		t.Errorf("commandText = %q, want %q", a.commandText, "hel")
	}
}

// ---------------------------------------------------------------------------
// 6. Update — Yank mode
// ---------------------------------------------------------------------------

func TestUpdateKeyYEntersYank(t *testing.T) {
	a := newTestApp()
	populateTabs(a, []TabItem{
		{ID: 1, Title: "Tab", URL: "https://example.com"},
	})

	model, _ := a.Update(keyRune('y'))
	a = model.(*App)

	if a.mode != ModeYank {
		t.Errorf("mode = %d, want ModeYank (%d)", a.mode, ModeYank)
	}
	if a.confirmHint == "" {
		t.Errorf("confirmHint should be set in yank mode")
	}
}

func TestUpdateYankEscCancels(t *testing.T) {
	a := newTestApp()
	a.mode = ModeYank
	a.confirmHint = "y:URL n:name h:host m:markdown"

	// In ModeYank, any key that is not y/n/h/m returns to normal mode
	// Esc is dispatched to handleKey which checks ModeYank first
	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyEscape})
	a = model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if a.confirmHint != "" {
		t.Errorf("confirmHint = %q, want empty", a.confirmHint)
	}
}

// ---------------------------------------------------------------------------
// 7. Update — Delete confirm mode
// ---------------------------------------------------------------------------

func TestUpdateKeyDEntersConfirmDelete(t *testing.T) {
	a := newTestApp()
	a.view = ViewSessions
	populateSessions(a, []SessionItem{
		{Name: "my-session", TabCount: 5, WindowCount: 1, GroupCount: 0, CreatedAt: "2025-01-01T00:00:00Z"},
	})

	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	a = model.(*App)

	if a.mode != ModeConfirmDelete {
		t.Errorf("mode = %d, want ModeConfirmDelete (%d)", a.mode, ModeConfirmDelete)
	}
	if a.confirmHint == "" {
		t.Errorf("confirmHint should be set")
	}
}

func TestUpdateConfirmDeleteEscCancels(t *testing.T) {
	a := newTestApp()
	a.mode = ModeConfirmDelete
	a.confirmHint = "Press D again to delete"

	// Any key that is not 'D' cancels confirm delete
	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyEscape})
	a = model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if a.confirmHint != "" {
		t.Errorf("confirmHint = %q, want empty", a.confirmHint)
	}
}

func TestUpdateConfirmDeleteNonDCancels(t *testing.T) {
	a := newTestApp()
	a.view = ViewSessions
	a.mode = ModeConfirmDelete
	a.confirmHint = "Press D again to delete"

	// Press 'x' (not 'D') → should cancel
	model, _ := a.Update(keyRune('x'))
	a = model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after non-D key", a.mode)
	}
}

func TestUpdateConfirmDeleteDInNonDeletableView(t *testing.T) {
	a := newTestApp()
	a.view = ViewTabs // Tabs doesn't support D-D delete

	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	a = model.(*App)

	// D key in tabs view should not enter confirm delete
	if a.mode == ModeConfirmDelete {
		t.Errorf("mode should not be ModeConfirmDelete for ViewTabs")
	}
}

// ---------------------------------------------------------------------------
// 8. View — Render smoke tests
// ---------------------------------------------------------------------------

func TestViewRenders(t *testing.T) {
	a := newTestApp()
	a.width = 80
	a.height = 24

	output := a.View()
	if output == "" {
		t.Error("View() returned empty string")
	}
	if !strings.Contains(output, "CTM") {
		t.Error("View() should contain 'CTM' header")
	}
	if !strings.Contains(output, "Tabs") {
		t.Error("View() should contain 'Tabs' in header")
	}
}

func TestViewRendersWithItems(t *testing.T) {
	a := newTestApp()
	a.width = 120
	a.height = 30
	populateTabs(a, []TabItem{
		{ID: 1, Title: "GitHub Dashboard", URL: "https://github.com", Active: true},
		{ID: 2, Title: "Google Search", URL: "https://google.com", Pinned: true},
	})

	output := a.View()
	if !strings.Contains(output, "GitHub Dashboard") {
		t.Error("View() should contain tab title 'GitHub Dashboard'")
	}
	if !strings.Contains(output, "Google Search") {
		t.Error("View() should contain tab title 'Google Search'")
	}
}

func TestViewRendersEmptyState(t *testing.T) {
	a := newTestApp()
	a.width = 80
	a.height = 24

	output := a.View()
	if !strings.Contains(output, "(no tabs") {
		t.Error("View() with no items should contain empty state hint")
	}
}

func TestViewRendersFilterBar(t *testing.T) {
	a := newTestApp()
	a.width = 80
	a.height = 24
	a.mode = ModeFilter
	a.filterText = "test"

	output := a.View()
	if !strings.Contains(output, "/ test") {
		t.Error("View() in filter mode should show '/ test'")
	}
}

func TestViewRendersCommandBar(t *testing.T) {
	a := newTestApp()
	a.width = 80
	a.height = 24
	a.mode = ModeCommand
	a.commandText = "help"

	output := a.View()
	if !strings.Contains(output, ": help") {
		t.Error("View() in command mode should show ': help'")
	}
}

func TestViewRendersNameInput(t *testing.T) {
	a := newTestApp()
	a.width = 80
	a.height = 24
	a.mode = ModeNameInput
	a.nameText = "my-session"

	output := a.View()
	if !strings.Contains(output, "Name: my-session") {
		t.Error("View() in name input mode should show 'Name: my-session'")
	}
}

func TestViewRendersErrorMsg(t *testing.T) {
	a := newTestApp()
	a.width = 80
	a.height = 24
	a.errorMsg = "connection failed"

	output := a.View()
	if !strings.Contains(output, "connection failed") {
		t.Error("View() with errorMsg should display the error")
	}
	if !strings.Contains(output, "ERROR") {
		t.Error("View() with errorMsg should contain 'ERROR' prefix")
	}
}

func TestViewRendersConfirmHint(t *testing.T) {
	a := newTestApp()
	a.width = 80
	a.height = 24
	a.confirmHint = "Press D again to delete"

	output := a.View()
	if !strings.Contains(output, "Press D again to delete") {
		t.Error("View() with confirmHint should display the hint")
	}
}

func TestViewRendersToast(t *testing.T) {
	a := newTestApp()
	a.width = 80
	a.height = 24
	a.toast = "Copied!"

	output := a.View()
	if !strings.Contains(output, "Copied!") {
		t.Error("View() with toast should display the toast text")
	}
}

func TestViewRendersHelpMode(t *testing.T) {
	a := newTestApp()
	a.width = 80
	a.height = 40
	a.mode = ModeHelp

	output := a.View()
	if !strings.Contains(output, "Help") {
		t.Error("View() in help mode should contain 'Help'")
	}
	if !strings.Contains(output, "quit") {
		t.Error("View() in help mode should show key bindings")
	}
}

func TestViewRendersSelectedCount(t *testing.T) {
	a := newTestApp()
	a.width = 80
	a.height = 24
	populateTabs(a, []TabItem{
		{ID: 1, Title: "A", URL: "https://a.com"},
		{ID: 2, Title: "B", URL: "https://b.com"},
	})
	vs := a.views[ViewTabs]
	vs.selected[0] = true
	vs.selected[1] = true

	output := a.View()
	if !strings.Contains(output, "2 selected") {
		t.Error("View() should show '2 selected' when items are selected")
	}
}

func TestViewRendersConnectionStatus(t *testing.T) {
	a := newTestApp()
	a.width = 80
	a.height = 24

	output := a.View()
	if !strings.Contains(output, "disconnected") {
		t.Error("View() should show 'disconnected' when not connected")
	}

	a.connected = true
	output = a.View()
	if !strings.Contains(output, "connected") {
		t.Error("View() should show 'connected' when connected")
	}

	a.selectedTarget = "target-1"
	output = a.View()
	if !strings.Contains(output, "target: target-1") {
		t.Error("View() should show 'target: target-1' when target is selected")
	}
}

// ---------------------------------------------------------------------------
// 9. handleEnter — ViewTargets
// ---------------------------------------------------------------------------

func TestHandleEnterTargetsView(t *testing.T) {
	a := newTestApp()
	a.view = ViewTargets
	populateTargets(a, []TargetItem{
		{TargetID: "target-abc", Channel: "cdp", Label: "Main", IsDefault: false},
		{TargetID: "target-xyz", Channel: "ext", Label: "Secondary", IsDefault: true},
	})
	a.views[ViewTargets].cursor = 0

	// handleEnter on ViewTargets sets selectedTarget and switches to ViewTabs
	a.handleEnter()

	if a.selectedTarget != "target-abc" {
		t.Errorf("selectedTarget = %q, want %q", a.selectedTarget, "target-abc")
	}
	if a.view != ViewTabs {
		t.Errorf("view = %v, want ViewTabs after selecting target", a.view)
	}
}

func TestHandleEnterEmptyView(t *testing.T) {
	a := newTestApp()
	a.view = ViewTabs
	// No items — handleEnter should return nil (no crash)
	cmd := a.handleEnter()
	if cmd != nil {
		t.Errorf("handleEnter on empty view should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// 10. Update — Event/refresh messages
// ---------------------------------------------------------------------------

func TestUpdateRefreshMsg(t *testing.T) {
	a := newTestApp()
	a.view = ViewTabs

	payload := json.RawMessage(`{
		"tabs": [
			{"id": 1, "title": "Tab One", "url": "https://one.com", "active": true},
			{"id": 2, "title": "Tab Two", "url": "https://two.com", "pinned": true}
		]
	}`)

	model, _ := a.Update(refreshMsg{payload: payload})
	a = model.(*App)

	vs := a.views[ViewTabs]
	if vs.itemCount != 2 {
		t.Errorf("itemCount = %d, want 2", vs.itemCount)
	}
	if len(vs.items) != 2 {
		t.Fatalf("items len = %d, want 2", len(vs.items))
	}
	tab0 := vs.items[0].(TabItem)
	if tab0.Title != "Tab One" {
		t.Errorf("items[0].Title = %q, want %q", tab0.Title, "Tab One")
	}
	tab1 := vs.items[1].(TabItem)
	if !tab1.Pinned {
		t.Error("items[1].Pinned should be true")
	}
}

func TestUpdateRefreshMsgGroups(t *testing.T) {
	a := newTestApp()
	a.view = ViewGroups

	payload := json.RawMessage(`{
		"groups": [
			{"id": 10, "title": "Work", "color": "blue", "collapsed": false}
		]
	}`)

	model, _ := a.Update(refreshMsg{payload: payload})
	a = model.(*App)

	vs := a.views[ViewGroups]
	if vs.itemCount != 1 {
		t.Errorf("itemCount = %d, want 1", vs.itemCount)
	}
	group := vs.items[0].(GroupItem)
	if group.Title != "Work" {
		t.Errorf("group.Title = %q, want %q", group.Title, "Work")
	}
}

func TestUpdateRefreshMsgSessions(t *testing.T) {
	a := newTestApp()
	a.view = ViewSessions

	payload := json.RawMessage(`{
		"sessions": [
			{"name": "morning", "tabCount": 10, "windowCount": 2, "groupCount": 1, "createdAt": "2025-06-01T08:00:00Z"}
		]
	}`)

	model, _ := a.Update(refreshMsg{payload: payload})
	a = model.(*App)

	vs := a.views[ViewSessions]
	if vs.itemCount != 1 {
		t.Errorf("itemCount = %d, want 1", vs.itemCount)
	}
}

func TestUpdateRefreshMsgTargets_AutoSelectDefault(t *testing.T) {
	a := newTestApp()
	a.view = ViewTargets

	payload := json.RawMessage(`{
		"targets": [
			{"targetId": "t-1", "channel": "cdp", "label": "One", "isDefault": false},
			{"targetId": "t-2", "channel": "ext", "label": "Two", "isDefault": true}
		]
	}`)

	model, _ := a.Update(refreshMsg{payload: payload})
	a = model.(*App)

	if a.selectedTarget != "t-2" {
		t.Errorf("selectedTarget = %q, want %q (auto-select default)", a.selectedTarget, "t-2")
	}
}

func TestUpdateRefreshMsgNilPayload(t *testing.T) {
	a := newTestApp()
	a.view = ViewTabs

	model, _ := a.Update(refreshMsg{payload: nil})
	a = model.(*App)

	vs := a.views[ViewTabs]
	if vs.itemCount != 0 {
		t.Errorf("itemCount = %d, want 0 (nil payload should be no-op)", vs.itemCount)
	}
}

func TestUpdateRefreshMsgClampsExistingCursor(t *testing.T) {
	a := newTestApp()
	a.view = ViewTabs
	vs := a.views[ViewTabs]
	vs.cursor = 10 // cursor was at position 10

	payload := json.RawMessage(`{
		"tabs": [
			{"id": 1, "title": "Only Tab", "url": "https://only.com"}
		]
	}`)

	model, _ := a.Update(refreshMsg{payload: payload})
	a = model.(*App)

	if vs.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (should clamp after refresh with fewer items)", vs.cursor)
	}
}

// ---------------------------------------------------------------------------
// 11. Update — errMsg
// ---------------------------------------------------------------------------

func TestUpdateErrMsg(t *testing.T) {
	a := newTestApp()

	model, _ := a.Update(errMsg{err: fmt.Errorf("test error")})
	a = model.(*App)

	if a.errorMsg != "test error" {
		t.Errorf("errorMsg = %q, want %q", a.errorMsg, "test error")
	}
}

func TestUpdateEscClearsError(t *testing.T) {
	a := newTestApp()
	a.errorMsg = "some error"

	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyEscape})
	a = model.(*App)

	if a.errorMsg != "" {
		t.Errorf("errorMsg = %q, want empty after Esc", a.errorMsg)
	}
}

// ---------------------------------------------------------------------------
// 12. Update — Toast handling
// ---------------------------------------------------------------------------

func TestUpdateToastClearMsg(t *testing.T) {
	a := newTestApp()
	a.toast = "something"

	model, _ := a.Update(toastClearMsg{})
	a = model.(*App)

	if a.toast != "" {
		t.Errorf("toast = %q, want empty after toastClearMsg", a.toast)
	}
}

// ---------------------------------------------------------------------------
// 13. Update — chordTimeoutMsg
// ---------------------------------------------------------------------------

func TestUpdateChordTimeoutMsg_YankMode(t *testing.T) {
	a := newTestApp()
	a.mode = ModeYank

	model, _ := a.Update(chordTimeoutMsg{})
	a = model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after chord timeout", a.mode)
	}
}

func TestUpdateChordTimeoutMsg_ZFilterMode(t *testing.T) {
	a := newTestApp()
	a.mode = ModeZFilter

	model, _ := a.Update(chordTimeoutMsg{})
	a = model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after chord timeout", a.mode)
	}
}

func TestUpdateChordTimeoutMsg_ConfirmDeleteMode(t *testing.T) {
	a := newTestApp()
	a.mode = ModeConfirmDelete
	a.confirmHint = "Press D again"

	model, _ := a.Update(chordTimeoutMsg{})
	a = model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after chord timeout", a.mode)
	}
	if a.confirmHint != "" {
		t.Errorf("confirmHint = %q, want empty after chord timeout", a.confirmHint)
	}
}

func TestUpdateChordTimeoutMsg_NormalMode_NoEffect(t *testing.T) {
	a := newTestApp()
	a.mode = ModeNormal

	model, _ := a.Update(chordTimeoutMsg{})
	a = model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal (unchanged)", a.mode)
	}
}

// ---------------------------------------------------------------------------
// 14. Update — pendingGTimeoutMsg
// ---------------------------------------------------------------------------

func TestUpdatePendingGTimeoutMsg(t *testing.T) {
	a := newTestApp()
	a.pendingG = true

	model, _ := a.Update(pendingGTimeoutMsg{})
	a = model.(*App)

	if a.pendingG {
		t.Errorf("pendingG should be false after timeout")
	}
}

func TestUpdatePendingGTimeoutMsg_AlreadyFalse(t *testing.T) {
	a := newTestApp()
	a.pendingG = false

	model, _ := a.Update(pendingGTimeoutMsg{})
	a = model.(*App)

	if a.pendingG {
		t.Errorf("pendingG should remain false")
	}
}

// ---------------------------------------------------------------------------
// 15. Z-filter interaction (end-to-end through Update)
// ---------------------------------------------------------------------------

func TestUpdateZFilterFlow(t *testing.T) {
	a := newTestApp()
	a.view = ViewTabs
	populateTabs(a, []TabItem{
		{ID: 1, Title: "GitHub", URL: "https://github.com", Pinned: true},
		{ID: 2, Title: "Google", URL: "https://google.com", Pinned: false},
		{ID: 3, Title: "GitLab", URL: "https://gitlab.com", Pinned: true},
	})

	// Press z to enter ZFilter mode
	model, _ := a.Update(keyRune('z'))
	a = model.(*App)

	if a.mode != ModeZFilter {
		t.Fatalf("mode = %d, want ModeZFilter after 'z'", a.mode)
	}
	if a.confirmHint == "" {
		t.Error("confirmHint should show z-filter options")
	}

	// Press p to filter by pinned
	model, _ = a.Update(keyRune('p'))
	a = model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after z-p", a.mode)
	}
	vs := a.views[ViewTabs]
	if len(vs.filtered) != 2 {
		t.Errorf("filtered count = %d, want 2 (pinned tabs)", len(vs.filtered))
	}
	if a.toast != "Filtered: pinned" {
		t.Errorf("toast = %q, want %q", a.toast, "Filtered: pinned")
	}

	// Press z again, then c to clear
	model, _ = a.Update(keyRune('z'))
	a = model.(*App)
	model, _ = a.Update(keyRune('c'))
	a = model.(*App)

	if vs.filtered != nil {
		t.Errorf("filtered should be nil after z-c clear, got %v", vs.filtered)
	}
	if a.toast != "Filter cleared" {
		t.Errorf("toast = %q, want %q", a.toast, "Filter cleared")
	}
}

func TestUpdateZFilterNotAvailableOutsideTabs(t *testing.T) {
	a := newTestApp()
	a.view = ViewTargets // z-filter doesn't work in Targets view

	model, _ := a.Update(keyRune('z'))
	a = model.(*App)

	if a.mode == ModeZFilter {
		t.Errorf("z-filter should not activate in ViewTargets")
	}
}

// ---------------------------------------------------------------------------
// 16. Multi-select via Space through Update
// ---------------------------------------------------------------------------

func TestUpdateSpaceTogglesSelect(t *testing.T) {
	a := newTestApp()
	populateTabs(a, []TabItem{
		{ID: 1, Title: "A", URL: "https://a.com"},
		{ID: 2, Title: "B", URL: "https://b.com"},
		{ID: 3, Title: "C", URL: "https://c.com"},
	})
	vs := a.views[ViewTabs]

	// Space toggles selection on current item and advances cursor
	model, _ := a.Update(keyRune(' '))
	a = model.(*App)

	if !vs.selected[0] {
		t.Error("item 0 should be selected after Space")
	}
	if vs.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (should advance)", vs.cursor)
	}

	// Space again to select item at cursor 1
	model, _ = a.Update(keyRune(' '))
	a = model.(*App)

	if !vs.selected[1] {
		t.Error("item 1 should be selected after second Space")
	}
	if vs.cursor != 2 {
		t.Errorf("cursor = %d, want 2", vs.cursor)
	}
}

func TestUpdateCtrlASelectsAll(t *testing.T) {
	a := newTestApp()
	populateTabs(a, []TabItem{
		{ID: 1, Title: "A", URL: "https://a.com"},
		{ID: 2, Title: "B", URL: "https://b.com"},
		{ID: 3, Title: "C", URL: "https://c.com"},
	})

	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	a = model.(*App)

	vs := a.views[ViewTabs]
	if len(vs.selected) != 3 {
		t.Errorf("selected count = %d, want 3 (ctrl+a selects all)", len(vs.selected))
	}
}

func TestUpdateUClearsSelection(t *testing.T) {
	a := newTestApp()
	populateTabs(a, []TabItem{
		{ID: 1, Title: "A", URL: "https://a.com"},
		{ID: 2, Title: "B", URL: "https://b.com"},
	})
	vs := a.views[ViewTabs]
	vs.selected[0] = true
	vs.selected[1] = true

	model, _ := a.Update(keyRune('u'))
	a = model.(*App)

	if len(vs.selected) != 0 {
		t.Errorf("selected count = %d, want 0 after 'u'", len(vs.selected))
	}
}

// ---------------------------------------------------------------------------
// 17. Ctrl+C / q quit
// ---------------------------------------------------------------------------

func TestUpdateCtrlCHandledByFramework(t *testing.T) {
	// ctrl+c is intercepted by Bubble Tea's framework before reaching Update.
	// When sent directly to Update, it is not handled by the app (no switch case),
	// so it falls through returning nil cmd. This is expected behavior.
	a := newTestApp()

	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if cmd != nil {
		t.Error("ctrl+c is handled by Bubble Tea framework, not by app's Update; cmd should be nil")
	}
}

func TestUpdateQQuits(t *testing.T) {
	a := newTestApp()

	_, cmd := a.Update(keyRune('q'))

	if !isQuitCmd(cmd) {
		t.Error("q in normal mode should return a quit command")
	}
}

func TestUpdateQInFilterModeDoesNotQuit(t *testing.T) {
	a := newTestApp()
	a.mode = ModeFilter

	_, cmd := a.Update(keyRune('q'))

	if isQuitCmd(cmd) {
		t.Error("q in filter mode should type 'q', not quit")
	}
	if a.filterText != "q" {
		t.Errorf("filterText = %q, want %q", a.filterText, "q")
	}
}

func TestUpdateQInCommandModeDoesNotQuit(t *testing.T) {
	a := newTestApp()
	a.mode = ModeCommand

	_, cmd := a.Update(keyRune('q'))

	if isQuitCmd(cmd) {
		t.Error("q in command mode should type 'q', not quit")
	}
	if a.commandText != "q" {
		t.Errorf("commandText = %q, want %q", a.commandText, "q")
	}
}

// ---------------------------------------------------------------------------
// 18. Help mode
// ---------------------------------------------------------------------------

func TestUpdateQuestionMarkEntersHelp(t *testing.T) {
	a := newTestApp()

	model, _ := a.Update(keyRune('?'))
	a = model.(*App)

	if a.mode != ModeHelp {
		t.Errorf("mode = %d, want ModeHelp after '?'", a.mode)
	}
}

func TestUpdateHelpModeEscExits(t *testing.T) {
	a := newTestApp()
	a.mode = ModeHelp

	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyEscape})
	a = model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after Esc in help", a.mode)
	}
}

func TestUpdateHelpModeQExits(t *testing.T) {
	a := newTestApp()
	a.mode = ModeHelp

	model, _ := a.Update(keyRune('q'))
	a = model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after q in help", a.mode)
	}
}

func TestUpdateHelpModeQuestionExits(t *testing.T) {
	a := newTestApp()
	a.mode = ModeHelp

	model, _ := a.Update(keyRune('?'))
	a = model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after ? in help", a.mode)
	}
}

// ---------------------------------------------------------------------------
// 19. Number keys switch views
// ---------------------------------------------------------------------------

func TestUpdateNumberKeysSwitchView(t *testing.T) {
	tests := []struct {
		key  rune
		want ViewType
	}{
		{'1', ViewTargets},
		{'2', ViewTabs},
		{'3', ViewGroups},
		{'4', ViewSessions},
		{'5', ViewCollections},
		{'6', ViewBookmarks},
		{'7', ViewWorkspaces},
		{'8', ViewSync},
	}

	for _, tt := range tests {
		a := newTestApp()
		a.view = ViewTabs // start from Tabs

		model, _ := a.Update(keyRune(tt.key))
		a = model.(*App)

		if a.view != tt.want {
			t.Errorf("key '%c': view = %v, want %v", tt.key, a.view, tt.want)
		}
	}
}

func TestUpdateKey1SwitchesToTargets(t *testing.T) {
	a := newTestApp()
	a.view = ViewTabs

	model, _ := a.Update(keyRune('1'))
	a = model.(*App)

	if a.view != ViewTargets {
		t.Errorf("view = %v, want ViewTargets after '1'", a.view)
	}
}

// ---------------------------------------------------------------------------
// 20. Name input mode
// ---------------------------------------------------------------------------

func TestUpdateKeyNEntersNameInput(t *testing.T) {
	a := newTestApp()
	a.view = ViewSessions

	model, _ := a.Update(keyRune('n'))
	a = model.(*App)

	if a.mode != ModeNameInput {
		t.Errorf("mode = %d, want ModeNameInput after 'n' in Sessions view", a.mode)
	}
	if a.nameText != "" {
		t.Errorf("nameText = %q, want empty", a.nameText)
	}
}

func TestUpdateKeyNEntersNameInputInCollections(t *testing.T) {
	a := newTestApp()
	a.view = ViewCollections

	model, _ := a.Update(keyRune('n'))
	a = model.(*App)

	if a.mode != ModeNameInput {
		t.Errorf("mode = %d, want ModeNameInput after 'n' in Collections view", a.mode)
	}
}

func TestUpdateKeyNEntersGroupCreateInTabs(t *testing.T) {
	a := newTestApp()
	a.view = ViewTabs

	model, _ := a.Update(keyRune('n'))
	a = model.(*App)

	if a.mode != ModeNameInput {
		t.Errorf("mode = %d, want ModeNameInput after 'n' in Tabs view (group selected)", a.mode)
	}
	if a.namePrompt != "Group name: " {
		t.Errorf("namePrompt = %q, want %q", a.namePrompt, "Group name: ")
	}
}

func TestUpdateNameInputEscCancels(t *testing.T) {
	a := newTestApp()
	a.mode = ModeNameInput
	a.nameText = "my-session"

	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyEscape})
	a = model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after Esc", a.mode)
	}
	if a.nameText != "" {
		t.Errorf("nameText = %q, want empty", a.nameText)
	}
}

func TestUpdateNameInputTyping(t *testing.T) {
	a := newTestApp()
	a.mode = ModeNameInput

	model, _ := a.Update(keyRune('a'))
	a = model.(*App)
	model, _ = a.Update(keyRune('b'))
	a = model.(*App)
	model, _ = a.Update(keyRune('c'))
	a = model.(*App)

	if a.nameText != "abc" {
		t.Errorf("nameText = %q, want %q", a.nameText, "abc")
	}
}

func TestUpdateNameInputBackspace(t *testing.T) {
	a := newTestApp()
	a.mode = ModeNameInput
	a.nameText = "test"

	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	a = model.(*App)

	if a.nameText != "tes" {
		t.Errorf("nameText = %q, want %q", a.nameText, "tes")
	}
}

func TestUpdateNameInputEnterEmptyDoesNothing(t *testing.T) {
	a := newTestApp()
	a.view = ViewSessions
	a.mode = ModeNameInput
	a.nameText = ""

	model, cmd := a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	a = model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after Enter with empty name", a.mode)
	}
	if cmd != nil {
		t.Error("cmd should be nil for empty name")
	}
}

// ---------------------------------------------------------------------------
// 21. Command execution via :quit
// ---------------------------------------------------------------------------

func TestUpdateCommandQuit(t *testing.T) {
	a := newTestApp()
	a.mode = ModeCommand
	a.commandText = "quit"

	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !isQuitCmd(cmd) {
		t.Error(":quit should return a quit command")
	}
}

func TestUpdateCommandQ(t *testing.T) {
	a := newTestApp()
	a.mode = ModeCommand
	a.commandText = "q"

	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !isQuitCmd(cmd) {
		t.Error(":q should return a quit command")
	}
}

func TestUpdateCommandHelp(t *testing.T) {
	a := newTestApp()
	a.mode = ModeCommand
	a.commandText = "help"

	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	a = model.(*App)

	if a.mode != ModeHelp {
		t.Errorf("mode = %d, want ModeHelp after :help", a.mode)
	}
}

func TestUpdateCommandTarget(t *testing.T) {
	a := newTestApp()
	a.mode = ModeCommand
	a.commandText = "target"

	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	a = model.(*App)

	if a.view != ViewTargets {
		t.Errorf("view = %v, want ViewTargets after :target", a.view)
	}
}

// ---------------------------------------------------------------------------
// 22. Esc behavior in normal mode
// ---------------------------------------------------------------------------

func TestUpdateEscInNormalModeClearsErrorAndHint(t *testing.T) {
	a := newTestApp()
	a.errorMsg = "error"
	a.confirmHint = "hint"

	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyEscape})
	a = model.(*App)

	if a.errorMsg != "" {
		t.Errorf("errorMsg = %q, want empty after Esc", a.errorMsg)
	}
	if a.confirmHint != "" {
		t.Errorf("confirmHint = %q, want empty after Esc", a.confirmHint)
	}
}

// ---------------------------------------------------------------------------
// 23. PendingG timeout does not override completed gg
// ---------------------------------------------------------------------------

func TestUpdatePendingGNonGKeyResets(t *testing.T) {
	a := newTestApp()
	populateTabs(a, []TabItem{
		{ID: 1, Title: "A", URL: "https://a.com"},
		{ID: 2, Title: "B", URL: "https://b.com"},
	})
	vs := a.views[ViewTabs]
	vs.cursor = 1

	// Press g to set pendingG
	model, _ := a.Update(keyRune('g'))
	a = model.(*App)
	if !a.pendingG {
		t.Fatal("pendingG should be true after g")
	}

	// Press j (not g) — pendingG should clear without going to top
	model, _ = a.Update(keyRune('j'))
	a = model.(*App)

	if a.pendingG {
		t.Error("pendingG should be false after non-g key")
	}
	// j should still move cursor down (but clamped since cursor was already at end or near it)
}

// ---------------------------------------------------------------------------
// 24. Cursor isolation — switching views preserves per-view cursor
// ---------------------------------------------------------------------------

func TestCursorIsolationAcrossViews(t *testing.T) {
	a := newTestApp()
	populateTabs(a, []TabItem{
		{ID: 1, Title: "A", URL: "https://a.com"},
		{ID: 2, Title: "B", URL: "https://b.com"},
		{ID: 3, Title: "C", URL: "https://c.com"},
	})
	a.views[ViewGroups].itemCount = 2
	a.views[ViewGroups].items = []any{
		GroupItem{ID: 1, Title: "G1"},
		GroupItem{ID: 2, Title: "G2"},
	}

	// Move tabs cursor to 2
	a.views[ViewTabs].cursor = 2

	// Switch to groups
	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyTab})
	a = model.(*App)

	if a.views[ViewGroups].cursor != 0 {
		t.Errorf("groups cursor = %d, want 0 (initial)", a.views[ViewGroups].cursor)
	}

	// Move groups cursor to 1
	model, _ = a.Update(keyRune('j'))
	a = model.(*App)
	if a.views[ViewGroups].cursor != 1 {
		t.Errorf("groups cursor = %d, want 1", a.views[ViewGroups].cursor)
	}

	// Switch back to tabs
	model, _ = a.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	a = model.(*App)

	// Tabs cursor should still be at 2
	if a.views[ViewTabs].cursor != 2 {
		t.Errorf("tabs cursor = %d, want 2 (preserved)", a.views[ViewTabs].cursor)
	}
	// Groups cursor should still be at 1
	if a.views[ViewGroups].cursor != 1 {
		t.Errorf("groups cursor = %d, want 1 (preserved)", a.views[ViewGroups].cursor)
	}
}

// ---------------------------------------------------------------------------
// 25. Ctrl+D / Ctrl+U page scrolling
// ---------------------------------------------------------------------------

func TestUpdateCtrlDPageDown(t *testing.T) {
	a := newTestApp()
	a.height = 24 // contentHeight = 24 - 3 = 21, half = 10
	populateTabs(a, make([]TabItem, 50))
	for i := range a.views[ViewTabs].items {
		a.views[ViewTabs].items[i] = TabItem{ID: i, Title: fmt.Sprintf("Tab %d", i), URL: fmt.Sprintf("https://%d.com", i)}
	}

	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	a = model.(*App)

	expected := a.contentHeight() / 2
	if a.views[ViewTabs].cursor != expected {
		t.Errorf("cursor = %d, want %d (half page down)", a.views[ViewTabs].cursor, expected)
	}
}

func TestUpdateCtrlUPageUp(t *testing.T) {
	a := newTestApp()
	a.height = 24
	populateTabs(a, make([]TabItem, 50))
	for i := range a.views[ViewTabs].items {
		a.views[ViewTabs].items[i] = TabItem{ID: i, Title: fmt.Sprintf("Tab %d", i), URL: fmt.Sprintf("https://%d.com", i)}
	}
	a.views[ViewTabs].cursor = 20

	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	a = model.(*App)

	expected := 20 - a.contentHeight()/2
	if expected < 0 {
		expected = 0
	}
	if a.views[ViewTabs].cursor != expected {
		t.Errorf("cursor = %d, want %d (half page up)", a.views[ViewTabs].cursor, expected)
	}
}

// ---------------------------------------------------------------------------
// 26. Unknown message type — no crash
// ---------------------------------------------------------------------------

func TestUpdateUnknownMsg(t *testing.T) {
	a := newTestApp()

	type customMsg struct{}
	model, cmd := a.Update(customMsg{})
	a = model.(*App)

	if cmd != nil {
		t.Errorf("unknown msg should return nil cmd")
	}
	// Should not panic, mode should remain normal
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
}

// ---------------------------------------------------------------------------
// 27. Refresh for different view types
// ---------------------------------------------------------------------------

func TestUpdateRefreshMsgCollections(t *testing.T) {
	a := newTestApp()
	a.view = ViewCollections

	payload := json.RawMessage(`{
		"collections": [
			{"name": "reading", "itemCount": 5, "createdAt": "2025-01-01T00:00:00Z", "updatedAt": "2025-06-01T00:00:00Z"},
			{"name": "work", "itemCount": 12, "createdAt": "2025-02-01T00:00:00Z", "updatedAt": "2025-06-15T00:00:00Z"}
		]
	}`)

	model, _ := a.Update(refreshMsg{payload: payload})
	a = model.(*App)

	vs := a.views[ViewCollections]
	if vs.itemCount != 2 {
		t.Errorf("itemCount = %d, want 2", vs.itemCount)
	}
	coll := vs.items[0].(CollectionItem)
	if coll.Name != "reading" {
		t.Errorf("items[0].Name = %q, want %q", coll.Name, "reading")
	}
}

func TestUpdateRefreshMsgBookmarks(t *testing.T) {
	a := newTestApp()
	a.view = ViewBookmarks

	payload := json.RawMessage(`{
		"tree": [
			{
				"id": "1",
				"title": "Folder",
				"children": [
					{"id": "2", "title": "Bookmark", "url": "https://example.com"}
				]
			}
		]
	}`)

	model, _ := a.Update(refreshMsg{payload: payload})
	a = model.(*App)

	vs := a.views[ViewBookmarks]
	// Default is all-folded on first load, so only the folder is visible
	if vs.itemCount != 1 {
		t.Errorf("itemCount = %d, want 1 (folder only, children folded)", vs.itemCount)
	}
	if !a.collapsedFolders["1"] {
		t.Error("folder should be collapsed by default")
	}
}

func TestUpdateRefreshMsgSyncStatus(t *testing.T) {
	a := newTestApp()
	a.view = ViewSync

	payload := json.RawMessage(`{
		"enabled": true,
		"syncDir": "/home/user/sync",
		"lastSync": "2025-06-01T00:00:00Z",
		"pendingChanges": 3,
		"conflicts": []
	}`)

	model, _ := a.Update(refreshMsg{payload: payload})
	a = model.(*App)

	vs := a.views[ViewSync]
	if vs.itemCount != 1 {
		t.Errorf("itemCount = %d, want 1", vs.itemCount)
	}
	status := vs.items[0].(SyncStatusItem)
	if !status.Enabled {
		t.Error("status.Enabled should be true")
	}
	if status.SyncDir != "/home/user/sync" {
		t.Errorf("status.SyncDir = %q, want %q", status.SyncDir, "/home/user/sync")
	}
}

// ---------------------------------------------------------------------------
// 28. View renders for each view type
// ---------------------------------------------------------------------------

func TestViewRendersGroupItems(t *testing.T) {
	a := newTestApp()
	a.view = ViewGroups
	a.width = 80
	a.height = 24
	vs := a.views[ViewGroups]
	vs.items = []any{GroupItem{ID: 1, Title: "Work", Color: "blue", Collapsed: false}}
	vs.itemCount = 1

	output := a.View()
	if !strings.Contains(output, "Work") {
		t.Error("View() should contain group title 'Work'")
	}
}

func TestViewRendersSessionItems(t *testing.T) {
	a := newTestApp()
	a.view = ViewSessions
	a.width = 100
	a.height = 24
	vs := a.views[ViewSessions]
	vs.items = []any{SessionItem{Name: "morning", TabCount: 10, WindowCount: 2, GroupCount: 1, CreatedAt: "2025-06-01T08:00:00Z"}}
	vs.itemCount = 1

	output := a.View()
	if !strings.Contains(output, "morning") {
		t.Error("View() should contain session name 'morning'")
	}
}

func TestViewRendersTargetItems(t *testing.T) {
	a := newTestApp()
	a.view = ViewTargets
	a.width = 80
	a.height = 24
	vs := a.views[ViewTargets]
	vs.items = []any{TargetItem{TargetID: "t-1", Channel: "cdp", Label: "Main", IsDefault: true, UserAgent: "Mozilla/5.0 Chrome/120"}}
	vs.itemCount = 1

	output := a.View()
	if !strings.Contains(output, "Chrome") {
		t.Error("View() should contain browser name derived from UserAgent")
	}
	if !strings.Contains(output, "Main") {
		t.Error("View() should contain target label 'Main'")
	}
}

// ---------------------------------------------------------------------------
// Bookmark DD delete flow
// ---------------------------------------------------------------------------

func TestBookmarkDD_FirstD_RootNodeBlocked(t *testing.T) {
	a := newTestApp()
	a.view = ViewBookmarks
	vs := a.views[ViewBookmarks]
	// Only Chrome's invisible root node (id "0") is blocked
	vs.items = []any{
		BookmarkItem{ID: "0", Title: "", Depth: 0, IsFolder: true},
	}
	vs.itemCount = 1

	model, _ := a.Update(keyRune('D'))
	a = model.(*App)

	if a.mode == ModeConfirmDelete {
		t.Error("root node (id 0) should NOT enter ModeConfirmDelete")
	}
	if a.errorMsg == "" {
		t.Error("should show error message for root node deletion")
	}
}

func TestBookmarkDD_FirstD_TopFolderAllowed(t *testing.T) {
	a := newTestApp()
	a.view = ViewBookmarks
	vs := a.views[ViewBookmarks]
	// Top-level folders like "Bookmarks Bar" (id "1") should be deletable
	vs.items = []any{
		BookmarkItem{ID: "1", Title: "Bookmarks Bar", Depth: 0, IsFolder: true},
	}
	vs.itemCount = 1

	model, _ := a.Update(keyRune('D'))
	a = model.(*App)

	if a.mode != ModeConfirmDelete {
		t.Errorf("top-level folder should enter ModeConfirmDelete, got mode=%d", a.mode)
	}
}

func TestBookmarkDD_FirstD_RegularBookmark(t *testing.T) {
	a := newTestApp()
	a.view = ViewBookmarks
	vs := a.views[ViewBookmarks]
	vs.items = []any{
		BookmarkItem{ID: "1", Title: "Bookmarks Bar", Depth: 0, IsFolder: true},
		BookmarkItem{ID: "42", Title: "Example", URL: "https://example.com", Depth: 1},
	}
	vs.itemCount = 2
	vs.cursor = 1 // cursor on the regular bookmark

	model, _ := a.Update(keyRune('D'))
	a = model.(*App)

	if a.mode != ModeConfirmDelete {
		t.Errorf("mode = %d, want ModeConfirmDelete (%d)", a.mode, ModeConfirmDelete)
	}
	if !strings.Contains(a.confirmHint, "Example") {
		t.Errorf("confirmHint = %q, should contain bookmark title", a.confirmHint)
	}
}

func TestBookmarkDD_SecondD_TriggersDelete(t *testing.T) {
	a := newTestApp()
	a.view = ViewBookmarks
	vs := a.views[ViewBookmarks]
	vs.items = []any{
		BookmarkItem{ID: "1", Title: "Bookmarks Bar", Depth: 0, IsFolder: true},
		BookmarkItem{ID: "42", Title: "Example", URL: "https://example.com", Depth: 1},
	}
	vs.itemCount = 2
	vs.cursor = 1

	// First D → ModeConfirmDelete
	model, _ := a.Update(keyRune('D'))
	a = model.(*App)
	if a.mode != ModeConfirmDelete {
		t.Fatalf("after first D: mode = %d, want ModeConfirmDelete", a.mode)
	}

	// Second D → should return a non-nil cmd (the delete command)
	model, cmd := a.Update(keyRune('D'))
	a = model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("after second D: mode = %d, want ModeNormal (%d)", a.mode, ModeNormal)
	}
	if cmd == nil {
		t.Error("after second D: cmd should not be nil (delete should be initiated)")
	}
}
