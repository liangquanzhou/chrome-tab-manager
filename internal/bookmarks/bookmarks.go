package bookmarks

import (
	"fmt"
	"strings"
)

// BookmarkNode represents a single bookmark or folder in the tree.
// If URL is non-empty, it is a bookmark. If Children is non-nil, it is a folder.
type BookmarkNode struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	URL       string          `json:"url,omitempty"`
	ParentID  string          `json:"parentId,omitempty"`
	DateAdded int64           `json:"dateAdded,omitempty"`
	Children  []*BookmarkNode `json:"children,omitempty"`
}

// IsFolder returns true if the node is a folder (has Children slice, even if empty).
func (n *BookmarkNode) IsFolder() bool {
	return n.Children != nil
}

// BookmarkOverlay stores CTM-specific metadata layered on top of a Chrome bookmark.
type BookmarkOverlay struct {
	BookmarkID string   `json:"bookmarkId"`
	Tags       []string `json:"tags"`
	Note       string   `json:"note,omitempty"`
	Alias      string   `json:"alias,omitempty"`
}

// BookmarkMirror is the locally cached snapshot of the full bookmark tree from a browser target.
type BookmarkMirror struct {
	Tree        []*BookmarkNode `json:"tree"`
	MirroredAt  string          `json:"mirroredAt"`
	TargetID    string          `json:"targetId"`
	NodeCount   int             `json:"nodeCount"`
	FolderCount int             `json:"folderCount"`
}

// CountNodes recursively counts all nodes and folders in the tree.
// A folder is a node with a non-nil Children slice.
func CountNodes(tree []*BookmarkNode) (nodes, folders int) {
	for _, n := range tree {
		nodes++
		if n.IsFolder() {
			folders++
			childNodes, childFolders := CountNodes(n.Children)
			nodes += childNodes
			folders += childFolders
		}
	}
	return nodes, folders
}

// SearchBookmarks searches the tree for bookmarks (not folders) whose title or URL
// contains the query string (case-insensitive). Returns a flat list of matching nodes.
func SearchBookmarks(tree []*BookmarkNode, query string) []*BookmarkNode {
	query = strings.ToLower(query)
	var results []*BookmarkNode
	searchRecursive(tree, query, &results)
	return results
}

func searchRecursive(tree []*BookmarkNode, query string, results *[]*BookmarkNode) {
	for _, n := range tree {
		if !n.IsFolder() {
			if strings.Contains(strings.ToLower(n.Title), query) ||
				strings.Contains(strings.ToLower(n.URL), query) {
				*results = append(*results, n)
			}
		}
		if n.Children != nil {
			searchRecursive(n.Children, query, results)
		}
	}
}

// FindNode searches the tree for a node with the given ID. Returns nil if not found.
func FindNode(tree []*BookmarkNode, id string) *BookmarkNode {
	for _, n := range tree {
		if n.ID == id {
			return n
		}
		if n.Children != nil {
			if found := FindNode(n.Children, id); found != nil {
				return found
			}
		}
	}
	return nil
}

// ExportMarkdown exports the bookmark tree as a Markdown-formatted string.
// Folders become headings (based on indent level), bookmarks become list items with links.
func ExportMarkdown(tree []*BookmarkNode, indent int) string {
	var b strings.Builder
	exportRecursive(tree, indent, &b)
	return b.String()
}

func exportRecursive(tree []*BookmarkNode, indent int, b *strings.Builder) {
	for _, n := range tree {
		if n.IsFolder() {
			if indent == 0 {
				fmt.Fprintf(b, "# %s\n\n", n.Title)
			} else if indent == 1 {
				fmt.Fprintf(b, "## %s\n\n", n.Title)
			} else {
				prefix := strings.Repeat("  ", indent-2)
				fmt.Fprintf(b, "%s- **%s**\n", prefix, n.Title)
			}
			exportRecursive(n.Children, indent+1, b)
			if indent <= 1 {
				b.WriteString("\n")
			}
		} else {
			prefix := strings.Repeat("  ", indent)
			if indent > 0 {
				prefix = strings.Repeat("  ", indent-1)
			}
			fmt.Fprintf(b, "%s- [%s](%s)\n", prefix, n.Title, n.URL)
		}
	}
}

// FlattenBookmarks returns a flat list of all bookmark nodes (non-folders) in the tree.
func FlattenBookmarks(tree []*BookmarkNode) []*BookmarkNode {
	var result []*BookmarkNode
	flattenRecursive(tree, &result)
	return result
}

func flattenRecursive(tree []*BookmarkNode, result *[]*BookmarkNode) {
	for _, n := range tree {
		if !n.IsFolder() {
			*result = append(*result, n)
		}
		if n.Children != nil {
			flattenRecursive(n.Children, result)
		}
	}
}
