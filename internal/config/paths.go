package config

import (
	"fmt"
	"os"
	"path/filepath"
)

func configBase() string {
	if dir := os.Getenv("CTM_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "ctm")
	}
	return filepath.Join(home, ".config", "ctm")
}

func ConfigDir() string        { return configBase() }
func SocketPath() string       { return filepath.Join(configBase(), "daemon.sock") }
func LockPath() string         { return filepath.Join(configBase(), "daemon.lock") }
func SessionsDir() string      { return filepath.Join(configBase(), "sessions") }
func CollectionsDir() string   { return filepath.Join(configBase(), "collections") }
func BookmarksDir() string     { return filepath.Join(configBase(), "bookmarks") }
func OverlaysDir() string      { return filepath.Join(configBase(), "overlays") }
func WorkspacesDir() string    { return filepath.Join(configBase(), "workspaces") }
func SavedSearchesDir() string { return filepath.Join(configBase(), "searches") }
func ExtensionDir() string     { return filepath.Join(configBase(), "extension") }
func SearchIndexPath() string  { return filepath.Join(configBase(), "search_index.json") }
func LogPath() string          { return filepath.Join(configBase(), "daemon.log") }

func SyncDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "ctm-sync")
	}
	return filepath.Join(home, "Library", "Mobile Documents", "com~ctm")
}

func EnsureDirs() error {
	dirs := []string{
		ConfigDir(),
		SessionsDir(),
		CollectionsDir(),
		BookmarksDir(),
		OverlaysDir(),
		WorkspacesDir(),
		SavedSearchesDir(),
		ExtensionDir(),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0700); err != nil {
			return fmt.Errorf("ensure dir %s: %w", d, err)
		}
	}
	return nil
}
