package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var bookmarksJSONOutput bool

var bookmarksCmd = &cobra.Command{
	Use:   "bookmarks",
	Short: "Manage browser bookmarks",
}

var bookmarksTreeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Display the full bookmark tree",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := connectAndRequest("bookmarks.tree", nil, targetSelector())
		if err != nil {
			return err
		}
		if bookmarksJSONOutput {
			printJSON(resp.Payload)
			return nil
		}

		var result struct {
			Tree []json.RawMessage `json:"tree"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		for _, raw := range result.Tree {
			printBookmarkTree(raw, 0)
		}
		return nil
	},
}

var bookmarksSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search bookmarks by title or URL",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"query": args[0]}
		resp, err := connectAndRequest("bookmarks.search", payload, targetSelector())
		if err != nil {
			return err
		}
		if bookmarksJSONOutput {
			printJSON(resp.Payload)
			return nil
		}

		var result struct {
			Bookmarks []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
				URL   string `json:"url"`
			} `json:"bookmarks"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTITLE\tURL")
		for _, b := range result.Bookmarks {
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				b.ID,
				truncate(b.Title, 40),
				truncate(b.URL, 60),
			)
		}
		return w.Flush()
	},
}

var bookmarksMirrorCmd = &cobra.Command{
	Use:   "mirror",
	Short: "Sync bookmark mirror from browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := connectAndRequest("bookmarks.mirror", nil, targetSelector())
		if err != nil {
			return err
		}
		if bookmarksJSONOutput {
			printJSON(resp.Payload)
			return nil
		}

		var result struct {
			NodeCount   int    `json:"nodeCount"`
			FolderCount int    `json:"folderCount"`
			MirroredAt  string `json:"mirroredAt"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		fmt.Printf("Bookmark mirror updated:\n")
		fmt.Printf("  Nodes:   %d\n", result.NodeCount)
		fmt.Printf("  Folders: %d\n", result.FolderCount)
		fmt.Printf("  Synced:  %s\n", result.MirroredAt)
		return nil
	},
}

var bookmarksExportFormat string

var bookmarksExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export bookmarks as markdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"format": bookmarksExportFormat}
		resp, err := connectAndRequest("bookmarks.export", payload, targetSelector())
		if err != nil {
			return err
		}
		if bookmarksJSONOutput {
			printJSON(resp.Payload)
			return nil
		}

		var result struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		fmt.Print(result.Content)
		return nil
	},
}

var bookmarksCreateParentID string

var bookmarksCreateCmd = &cobra.Command{
	Use:   "create <title> [url]",
	Short: "Create a new bookmark (folder if no URL given)",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"title": args[0]}
		if len(args) > 1 {
			payload["url"] = args[1]
		}
		if bookmarksCreateParentID != "" {
			payload["parentId"] = bookmarksCreateParentID
		}
		resp, err := connectAndRequest("bookmarks.create", payload, targetSelector())
		if err != nil {
			return err
		}
		if bookmarksJSONOutput {
			printJSON(resp.Payload)
			return nil
		}
		fmt.Printf("Bookmark %q created.\n", args[0])
		return nil
	},
}

var bookmarksRemoveCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove a bookmark or folder from Chrome",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := connectAndRequest("bookmarks.remove", map[string]string{"id": args[0]}, targetSelector())
		if err != nil {
			return err
		}
		fmt.Printf("Bookmark %q removed.\n", args[0])
		return nil
	},
}

func init() {
	bookmarksCmd.PersistentFlags().BoolVar(&bookmarksJSONOutput, "json", false, "Output as JSON")

	bookmarksExportCmd.Flags().StringVar(&bookmarksExportFormat, "format", "markdown", "Export format (markdown)")

	bookmarksCreateCmd.Flags().StringVar(&bookmarksCreateParentID, "parent", "", "Parent folder ID")

	bookmarksCmd.AddCommand(bookmarksTreeCmd)
	bookmarksCmd.AddCommand(bookmarksSearchCmd)
	bookmarksCmd.AddCommand(bookmarksMirrorCmd)
	bookmarksCmd.AddCommand(bookmarksExportCmd)
	bookmarksCmd.AddCommand(bookmarksCreateCmd)
	bookmarksCmd.AddCommand(bookmarksRemoveCmd)
	rootCmd.AddCommand(bookmarksCmd)
}

// printBookmarkTree recursively prints a bookmark tree node with indentation.
func printBookmarkTree(raw json.RawMessage, depth int) {
	var node struct {
		ID       string            `json:"id"`
		Title    string            `json:"title"`
		URL      string            `json:"url"`
		Children []json.RawMessage `json:"children"`
	}
	if err := json.Unmarshal(raw, &node); err != nil {
		return
	}

	indent := strings.Repeat("  ", depth)
	if len(node.Children) > 0 || node.URL == "" {
		// Folder
		fmt.Printf("%s[F] %s\n", indent, node.Title)
		for _, child := range node.Children {
			printBookmarkTree(child, depth+1)
		}
	} else {
		// Bookmark
		fmt.Printf("%s%s  %s\n", indent, node.Title, styleDimCLI(node.URL))
	}
}

// styleDimCLI returns a dimmed string for CLI output (using ANSI codes).
func styleDimCLI(s string) string {
	return "\033[2m" + s + "\033[0m"
}
