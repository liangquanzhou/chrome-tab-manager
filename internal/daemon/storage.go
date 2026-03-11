package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var nameRe = regexp.MustCompile(`^[\p{L}\p{N}_-]+$`)

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if len([]rune(name)) > 128 {
		return fmt.Errorf("name too long (max 128 characters)")
	}
	if !nameRe.MatchString(name) {
		return fmt.Errorf("name contains invalid characters (allowed: letters, digits, _ -)")
	}
	return nil
}

// validatePathSafe ensures a string is safe for use as a filename component.
// More permissive than validateName (allows dots, longer IDs) but still blocks
// path traversal. Use for external IDs like Chrome bookmark IDs and saved search IDs.
func validatePathSafe(id string) error {
	if id == "" {
		return fmt.Errorf("id cannot be empty")
	}
	if len(id) > 256 {
		return fmt.Errorf("id too long (max 256 characters)")
	}
	if strings.ContainsAny(id, "/\\") {
		return fmt.Errorf("id contains path separator")
	}
	if id == "." || id == ".." || strings.Contains(id, "..") {
		return fmt.Errorf("id contains path traversal sequence")
	}
	return nil
}

func atomicWriteJSON(dir, filename string, data any) error {
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	target := filepath.Join(dir, filename)
	// Verify target stays within dir (path traversal defense)
	// Use filepath.Rel to correctly handle sibling directory edge cases
	// (e.g., dir="/a/b" target="/a/bc" would pass a prefix check but fail Rel)
	cleanDir := filepath.Clean(dir) + string(filepath.Separator)
	cleanTarget := filepath.Clean(target)
	if !strings.HasPrefix(cleanTarget, cleanDir) {
		return fmt.Errorf("path traversal detected")
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()

	defer func() {
		// Clean up temp file on error
		if tmpPath != "" {
			os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("fsync: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}

	if err := os.Rename(tmpPath, target); err != nil {
		return fmt.Errorf("rename: %w", err)
	}

	tmpPath = "" // prevent cleanup
	return nil
}

func loadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("unmarshal %s: %w", path, err)
	}
	return nil
}

func listJSONFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") && !strings.HasPrefix(e.Name(), ".tmp-") {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
