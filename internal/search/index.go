package search

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// SearchIndex maintains a lightweight in-memory + on-disk index of searchable resources.
type SearchIndex struct {
	mu      sync.RWMutex
	entries map[string]*IndexEntry // key = "kind:id"
	dirty   bool
	path    string
}

// IndexEntry represents a single indexed resource.
type IndexEntry struct {
	Kind      string    `json:"kind"`      // "session", "collection", "bookmark", "workspace"
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	URL       string    `json:"url,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
	Checksum  string    `json:"checksum"` // detect changes without re-reading
}

type indexFile struct {
	Version int                    `json:"version"`
	Entries map[string]*IndexEntry `json:"entries"`
	BuiltAt time.Time              `json:"builtAt"`
}

// NewSearchIndex creates a new search index that persists to indexPath.
func NewSearchIndex(indexPath string) *SearchIndex {
	return &SearchIndex{
		entries: make(map[string]*IndexEntry),
		path:    indexPath,
	}
}

// Load reads the index from disk. Returns nil if the file does not exist yet.
func (si *SearchIndex) Load() error {
	si.mu.Lock()
	defer si.mu.Unlock()

	data, err := os.ReadFile(si.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no index yet
		}
		return fmt.Errorf("read index: %w", err)
	}

	var idx indexFile
	if err := json.Unmarshal(data, &idx); err != nil {
		return fmt.Errorf("unmarshal index: %w", err)
	}

	if idx.Version != 1 {
		// Unknown version; start fresh rather than failing
		si.entries = make(map[string]*IndexEntry)
		si.dirty = true
		return nil
	}

	si.entries = idx.Entries
	if si.entries == nil {
		si.entries = make(map[string]*IndexEntry)
	}
	return nil
}

// Save writes the index to disk atomically. No-op if the index has not been modified.
func (si *SearchIndex) Save() error {
	si.mu.Lock()
	defer si.mu.Unlock()

	if !si.dirty {
		return nil
	}

	idx := indexFile{
		Version: 1,
		Entries: si.entries,
		BuiltAt: time.Now(),
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}

	// Atomic write: tmp → fsync → rename
	tmp := si.path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create temp index: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("write temp index: %w", err)
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("fsync temp index: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("close temp index: %w", err)
	}

	if err := os.Rename(tmp, si.path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename index: %w", err)
	}

	si.dirty = false
	return nil
}

// Upsert adds or updates an entry in the index.
func (si *SearchIndex) Upsert(entry *IndexEntry) {
	if entry == nil {
		return
	}
	si.mu.Lock()
	defer si.mu.Unlock()
	key := entry.Kind + ":" + entry.ID
	si.entries[key] = entry
	si.dirty = true
}

// Remove deletes an entry from the index.
func (si *SearchIndex) Remove(kind, id string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	key := kind + ":" + id
	if _, ok := si.entries[key]; ok {
		delete(si.entries, key)
		si.dirty = true
	}
}

// RemoveByKind removes all entries of a given kind from the index.
func (si *SearchIndex) RemoveByKind(kind string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	prefix := kind + ":"
	for key := range si.entries {
		if strings.HasPrefix(key, prefix) {
			delete(si.entries, key)
			si.dirty = true
		}
	}
}

// Search returns entries matching the query string, filtered by optional kinds.
// Uses the same Match logic as the existing search engine (case-insensitive substring on title/URL/tags).
// Results are sorted by score descending.
func (si *SearchIndex) Search(query string, kinds []string) []*IndexEntry {
	si.mu.RLock()
	defer si.mu.RUnlock()

	kindSet := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		kindSet[k] = true
	}

	type scored struct {
		entry *IndexEntry
		score float64
	}

	var matches []scored

	for _, entry := range si.entries {
		// Filter by kind if specified
		if len(kindSet) > 0 && !kindSet[entry.Kind] {
			continue
		}

		bestScore := 0.0

		// Match against title
		if ok, s := Match(query, entry.Title); ok && s > bestScore {
			bestScore = s
		}

		// Match against URL
		if ok, s := Match(query, entry.URL); ok && s > bestScore {
			bestScore = s
		}

		// Match against tags
		for _, tag := range entry.Tags {
			if ok, s := Match(query, tag); ok && s > bestScore {
				bestScore = s
			}
		}

		if bestScore > 0 {
			matches = append(matches, scored{entry: entry, score: bestScore})
		}
	}

	// Sort by score descending
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})

	result := make([]*IndexEntry, len(matches))
	for i, m := range matches {
		result[i] = m.entry
	}
	return result
}

// Len returns the number of entries in the index.
func (si *SearchIndex) Len() int {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return len(si.entries)
}

// IsDirty returns whether the index has unsaved changes.
func (si *SearchIndex) IsDirty() bool {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.dirty
}

// Checksum computes a SHA256 checksum for content change detection.
// The result is truncated to 16 hex characters for efficiency.
func Checksum(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8])
}
