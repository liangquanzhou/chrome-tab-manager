package bookmarks

import (
	"strings"
	"testing"
)

func sampleTree() []*BookmarkNode {
	return []*BookmarkNode{
		{
			ID:    "0",
			Title: "Root",
			Children: []*BookmarkNode{
				{
					ID:    "1",
					Title: "Bookmarks Bar",
					Children: []*BookmarkNode{
						{
							ID:       "10",
							Title:    "GitHub",
							URL:      "https://github.com",
							ParentID: "1",
						},
						{
							ID:       "11",
							Title:    "Google",
							URL:      "https://www.google.com",
							ParentID: "1",
						},
						{
							ID:    "12",
							Title: "Work",
							Children: []*BookmarkNode{
								{
									ID:       "120",
									Title:    "Jira",
									URL:      "https://jira.example.com",
									ParentID: "12",
								},
								{
									ID:       "121",
									Title:    "Confluence",
									URL:      "https://wiki.example.com",
									ParentID: "12",
								},
							},
							ParentID: "1",
						},
					},
				},
				{
					ID:    "2",
					Title: "Other Bookmarks",
					Children: []*BookmarkNode{
						{
							ID:       "20",
							Title:    "Stack Overflow",
							URL:      "https://stackoverflow.com",
							ParentID: "2",
						},
					},
				},
			},
		},
	}
}

func TestCountNodes_EmptyTree(t *testing.T) {
	nodes, folders := CountNodes(nil)
	if nodes != 0 || folders != 0 {
		t.Errorf("CountNodes(nil) = (%d, %d), want (0, 0)", nodes, folders)
	}

	nodes, folders = CountNodes([]*BookmarkNode{})
	if nodes != 0 || folders != 0 {
		t.Errorf("CountNodes([]) = (%d, %d), want (0, 0)", nodes, folders)
	}
}

func TestCountNodes_NestedTree(t *testing.T) {
	tree := sampleTree()
	nodes, folders := CountNodes(tree)

	// Total nodes: Root(1) + Bookmarks Bar(1) + GitHub(1) + Google(1) + Work(1) +
	//              Jira(1) + Confluence(1) + Other Bookmarks(1) + Stack Overflow(1) = 9
	// Folders: Root(1) + Bookmarks Bar(1) + Work(1) + Other Bookmarks(1) = 4
	if nodes != 9 {
		t.Errorf("CountNodes nodes = %d, want 9", nodes)
	}
	if folders != 4 {
		t.Errorf("CountNodes folders = %d, want 4", folders)
	}
}

func TestCountNodes_SingleBookmark(t *testing.T) {
	tree := []*BookmarkNode{
		{ID: "1", Title: "Test", URL: "https://test.com"},
	}
	nodes, folders := CountNodes(tree)
	if nodes != 1 || folders != 0 {
		t.Errorf("CountNodes single bookmark = (%d, %d), want (1, 0)", nodes, folders)
	}
}

func TestCountNodes_EmptyFolder(t *testing.T) {
	tree := []*BookmarkNode{
		{ID: "1", Title: "Empty Folder", Children: []*BookmarkNode{}},
	}
	nodes, folders := CountNodes(tree)
	if nodes != 1 || folders != 1 {
		t.Errorf("CountNodes empty folder = (%d, %d), want (1, 1)", nodes, folders)
	}
}

func TestSearchBookmarks_MatchTitle(t *testing.T) {
	tree := sampleTree()
	results := SearchBookmarks(tree, "github")
	if len(results) != 1 {
		t.Fatalf("SearchBookmarks('github') = %d results, want 1", len(results))
	}
	if results[0].Title != "GitHub" {
		t.Errorf("SearchBookmarks('github')[0].Title = %q, want %q", results[0].Title, "GitHub")
	}
}

func TestSearchBookmarks_MatchURL(t *testing.T) {
	tree := sampleTree()
	results := SearchBookmarks(tree, "stackoverflow")
	if len(results) != 1 {
		t.Fatalf("SearchBookmarks('stackoverflow') = %d results, want 1", len(results))
	}
	if results[0].Title != "Stack Overflow" {
		t.Errorf("got title %q, want %q", results[0].Title, "Stack Overflow")
	}
}

func TestSearchBookmarks_CaseInsensitive(t *testing.T) {
	tree := sampleTree()
	results := SearchBookmarks(tree, "GITHUB")
	if len(results) != 1 {
		t.Fatalf("SearchBookmarks('GITHUB') = %d results, want 1", len(results))
	}
}

func TestSearchBookmarks_NoMatch(t *testing.T) {
	tree := sampleTree()
	results := SearchBookmarks(tree, "nonexistent")
	if len(results) != 0 {
		t.Errorf("SearchBookmarks('nonexistent') = %d results, want 0", len(results))
	}
}

func TestSearchBookmarks_MultipleMatches(t *testing.T) {
	tree := sampleTree()
	// "example" matches jira.example.com and wiki.example.com
	results := SearchBookmarks(tree, "example")
	if len(results) != 2 {
		t.Errorf("SearchBookmarks('example') = %d results, want 2", len(results))
	}
}

func TestSearchBookmarks_SkipsFolders(t *testing.T) {
	tree := sampleTree()
	// "Work" is a folder title, should not be returned
	results := SearchBookmarks(tree, "Work")
	if len(results) != 0 {
		t.Errorf("SearchBookmarks('Work') should skip folders, got %d results", len(results))
	}
}

func TestFindNode_Exists(t *testing.T) {
	tree := sampleTree()
	node := FindNode(tree, "120")
	if node == nil {
		t.Fatal("FindNode('120') = nil, want non-nil")
	}
	if node.Title != "Jira" {
		t.Errorf("FindNode('120').Title = %q, want %q", node.Title, "Jira")
	}
}

func TestFindNode_Root(t *testing.T) {
	tree := sampleTree()
	node := FindNode(tree, "0")
	if node == nil {
		t.Fatal("FindNode('0') = nil, want non-nil")
	}
	if node.Title != "Root" {
		t.Errorf("FindNode('0').Title = %q, want %q", node.Title, "Root")
	}
}

func TestFindNode_NotExists(t *testing.T) {
	tree := sampleTree()
	node := FindNode(tree, "999")
	if node != nil {
		t.Errorf("FindNode('999') = %v, want nil", node)
	}
}

func TestFindNode_EmptyTree(t *testing.T) {
	node := FindNode(nil, "1")
	if node != nil {
		t.Errorf("FindNode on nil tree = %v, want nil", node)
	}
}

func TestExportMarkdown_Format(t *testing.T) {
	tree := []*BookmarkNode{
		{
			ID:    "1",
			Title: "Bookmarks Bar",
			Children: []*BookmarkNode{
				{ID: "10", Title: "GitHub", URL: "https://github.com"},
				{ID: "11", Title: "Google", URL: "https://google.com"},
				{
					ID:    "12",
					Title: "Dev",
					Children: []*BookmarkNode{
						{ID: "120", Title: "MDN", URL: "https://developer.mozilla.org"},
					},
				},
			},
		},
	}

	md := ExportMarkdown(tree, 0)

	// Should contain the folder as heading
	if !strings.Contains(md, "# Bookmarks Bar") {
		t.Errorf("ExportMarkdown missing '# Bookmarks Bar' heading")
	}
	// Should contain bookmarks as links
	if !strings.Contains(md, "[GitHub](https://github.com)") {
		t.Errorf("ExportMarkdown missing GitHub link")
	}
	if !strings.Contains(md, "[Google](https://google.com)") {
		t.Errorf("ExportMarkdown missing Google link")
	}
	// Sub-folder should be a sub-heading
	if !strings.Contains(md, "## Dev") {
		t.Errorf("ExportMarkdown missing '## Dev' sub-heading")
	}
	if !strings.Contains(md, "[MDN](https://developer.mozilla.org)") {
		t.Errorf("ExportMarkdown missing MDN link")
	}
}

func TestExportMarkdown_EmptyTree(t *testing.T) {
	md := ExportMarkdown(nil, 0)
	if md != "" {
		t.Errorf("ExportMarkdown(nil) = %q, want empty", md)
	}
}

func TestFlattenBookmarks_FiltersOutFolders(t *testing.T) {
	tree := sampleTree()
	flat := FlattenBookmarks(tree)

	// Should contain: GitHub, Google, Jira, Confluence, Stack Overflow = 5
	if len(flat) != 5 {
		t.Errorf("FlattenBookmarks count = %d, want 5", len(flat))
	}

	for _, n := range flat {
		if n.IsFolder() {
			t.Errorf("FlattenBookmarks should not contain folders, got folder %q", n.Title)
		}
		if n.URL == "" {
			t.Errorf("FlattenBookmarks node %q has empty URL", n.Title)
		}
	}
}

func TestFlattenBookmarks_EmptyTree(t *testing.T) {
	flat := FlattenBookmarks(nil)
	if len(flat) != 0 {
		t.Errorf("FlattenBookmarks(nil) = %d items, want 0", len(flat))
	}
}

func TestFlattenBookmarks_PreservesOrder(t *testing.T) {
	tree := sampleTree()
	flat := FlattenBookmarks(tree)

	expected := []string{"GitHub", "Google", "Jira", "Confluence", "Stack Overflow"}
	if len(flat) != len(expected) {
		t.Fatalf("FlattenBookmarks count = %d, want %d", len(flat), len(expected))
	}
	for i, n := range flat {
		if n.Title != expected[i] {
			t.Errorf("FlattenBookmarks[%d].Title = %q, want %q", i, n.Title, expected[i])
		}
	}
}

func TestIsFolder(t *testing.T) {
	folder := &BookmarkNode{ID: "1", Title: "Folder", Children: []*BookmarkNode{}}
	if !folder.IsFolder() {
		t.Error("node with empty Children should be a folder")
	}

	bookmark := &BookmarkNode{ID: "2", Title: "Bookmark", URL: "https://example.com"}
	if bookmark.IsFolder() {
		t.Error("node with URL and no Children should not be a folder")
	}
}
