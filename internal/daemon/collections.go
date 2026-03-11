package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"ctm/internal/protocol"
)

type CollectionItem struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	GroupLabel string `json:"groupLabel,omitempty"`
}

type Collection struct {
	Name      string           `json:"name"`
	CreatedAt string           `json:"createdAt"`
	UpdatedAt string           `json:"updatedAt"`
	Items     []CollectionItem `json:"items"`
}

type CollectionSummary struct {
	Name      string `json:"name"`
	ItemCount int    `json:"itemCount"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

func (c *Collection) Summary() CollectionSummary {
	return CollectionSummary{
		Name:      c.Name,
		ItemCount: len(c.Items),
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

func (h *Hub) handleCollectionsAction(ctx context.Context, incoming *incomingMessage) {
	switch incoming.msg.Action {
	case "collections.list":
		h.handleCollectionsList(incoming)
	case "collections.get":
		h.handleCollectionsGet(incoming)
	case "collections.create":
		h.handleCollectionsCreate(incoming)
	case "collections.delete":
		h.handleCollectionsDelete(incoming)
	case "collections.addItems":
		h.handleCollectionsAddItems(incoming)
	case "collections.rename":
		h.handleCollectionsRename(incoming)
	case "collections.removeItems":
		h.handleCollectionsRemoveItems(incoming)
	case "collections.reorder":
		h.handleCollectionsReorder(incoming)
	case "collections.restore":
		h.handleCollectionsRestore(ctx, incoming)
	default:
		err := &UnknownActionError{Action: incoming.msg.Action}
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(err), err.Error())
	}
}

func (h *Hub) handleCollectionsList(incoming *incomingMessage) {
	files, err := listJSONFiles(h.collectionsDir)
	if err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("list collections: %s", err))
		return
	}

	summaries := make([]CollectionSummary, 0, len(files))
	for _, f := range files {
		var c Collection
		if err := loadJSON(filepath.Join(h.collectionsDir, f), &c); err != nil {
			continue
		}
		summaries = append(summaries, c.Summary())
	}

	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{"collections": summaries})
}

func (h *Hub) handleCollectionsGet(incoming *incomingMessage) {
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid payload")
		return
	}

	if err := validateName(payload.Name); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, err.Error())
		return
	}

	path := filepath.Join(h.collectionsDir, payload.Name+".json")
	var c Collection
	if err := loadJSON(path, &c); err != nil {
		notFound := &ResourceNotFoundError{Kind: "collection", Name: payload.Name, Err: ErrCollectionNotFound}
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(notFound), notFound.Error())
		return
	}

	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{"collection": c})
}

func (h *Hub) handleCollectionsCreate(incoming *incomingMessage) {
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid payload")
		return
	}

	if err := validateName(payload.Name); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, err.Error())
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	c := &Collection{
		Name:      payload.Name,
		CreatedAt: now,
		UpdatedAt: now,
		Items:     []CollectionItem{},
	}

	if err := atomicWriteJSON(h.collectionsDir, payload.Name+".json", c); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("create failed: %s", err))
		return
	}

	h.indexCollection(c)
	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{
		"name":      c.Name,
		"createdAt": c.CreatedAt,
	})
}

func (h *Hub) handleCollectionsDelete(incoming *incomingMessage) {
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid payload")
		return
	}

	if err := validateName(payload.Name); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, err.Error())
		return
	}

	path := filepath.Join(h.collectionsDir, payload.Name+".json")
	if err := os.Remove(path); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("delete failed: %s", err))
		return
	}

	h.removeFromIndex("collection", payload.Name)
	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{})
}

func (h *Hub) handleCollectionsAddItems(incoming *incomingMessage) {
	var payload struct {
		Name  string           `json:"name"`
		Items []CollectionItem `json:"items"`
	}
	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid payload")
		return
	}

	if err := validateName(payload.Name); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, err.Error())
		return
	}

	path := filepath.Join(h.collectionsDir, payload.Name+".json")
	var c Collection
	if err := loadJSON(path, &c); err != nil {
		notFound := &ResourceNotFoundError{Kind: "collection", Name: payload.Name, Err: ErrCollectionNotFound}
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(notFound), notFound.Error())
		return
	}

	// Deduplicate: skip items whose URL already exists in collection
	existing := make(map[string]bool, len(c.Items))
	for _, item := range c.Items {
		existing[item.URL] = true
	}
	added := 0
	skipped := 0
	for _, item := range payload.Items {
		if existing[item.URL] {
			skipped++
			continue
		}
		existing[item.URL] = true
		c.Items = append(c.Items, item)
		added++
	}

	c.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := atomicWriteJSON(h.collectionsDir, payload.Name+".json", &c); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("save failed: %s", err))
		return
	}

	h.indexCollection(&c)
	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{
		"name":      c.Name,
		"itemCount": len(c.Items),
		"added":     added,
		"skipped":   skipped,
	})
}

func (h *Hub) handleCollectionsRestore(ctx context.Context, incoming *incomingMessage) {
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid payload")
		return
	}

	if err := validateName(payload.Name); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, err.Error())
		return
	}

	path := filepath.Join(h.collectionsDir, payload.Name+".json")
	var c Collection
	if err := loadJSON(path, &c); err != nil {
		notFound := &ResourceNotFoundError{Kind: "collection", Name: payload.Name, Err: ErrCollectionNotFound}
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(notFound), notFound.Error())
		return
	}

	target, err := h.resolveTarget(incoming.msg.Target)
	if err != nil {
		sendError(incoming.writer, incoming.msg.ID, targetErrorCode(err), err.Error())
		return
	}

	go func() {
		tabsOpened := 0
		tabsFailed := 0

		for _, item := range c.Items {
			_, err := h.sendToExtensionAndWait(ctx, target, "tabs.open", mustMarshal(map[string]any{
				"url":    item.URL,
				"active": false,
			}))
			if err != nil {
				tabsFailed++
			} else {
				tabsOpened++
			}
		}

		// Focus the browser window so user can see what was restored
		h.focusBrowserWindow(ctx, target)

		sendResponse(incoming.writer, incoming.msg.ID, map[string]any{
			"tabsOpened": tabsOpened,
			"tabsFailed": tabsFailed,
		})
	}()
}

func (h *Hub) handleCollectionsRemoveItems(incoming *incomingMessage) {
	var payload struct {
		Name string   `json:"name"`
		URLs []string `json:"urls"`
	}
	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid payload")
		return
	}

	if err := validateName(payload.Name); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, err.Error())
		return
	}

	path := filepath.Join(h.collectionsDir, payload.Name+".json")
	var c Collection
	if err := loadJSON(path, &c); err != nil {
		notFound := &ResourceNotFoundError{Kind: "collection", Name: payload.Name, Err: ErrCollectionNotFound}
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(notFound), notFound.Error())
		return
	}

	removeSet := make(map[string]bool, len(payload.URLs))
	for _, u := range payload.URLs {
		removeSet[u] = true
	}

	filtered := c.Items[:0]
	for _, item := range c.Items {
		if !removeSet[item.URL] {
			filtered = append(filtered, item)
		}
	}
	c.Items = filtered
	c.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := atomicWriteJSON(h.collectionsDir, payload.Name+".json", &c); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("save failed: %s", err))
		return
	}

	h.indexCollection(&c)
	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{
		"name":      c.Name,
		"itemCount": len(c.Items),
	})
}

func (h *Hub) handleCollectionsRename(incoming *incomingMessage) {
	var payload struct {
		Name    string `json:"name"`
		NewName string `json:"newName"`
	}
	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid payload")
		return
	}

	if err := validateName(payload.Name); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, err.Error())
		return
	}
	if err := validateName(payload.NewName); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, err.Error())
		return
	}

	oldPath := filepath.Join(h.collectionsDir, payload.Name+".json")
	var c Collection
	if err := loadJSON(oldPath, &c); err != nil {
		notFound := &ResourceNotFoundError{Kind: "collection", Name: payload.Name, Err: ErrCollectionNotFound}
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(notFound), notFound.Error())
		return
	}

	// Check new name doesn't already exist
	newPath := filepath.Join(h.collectionsDir, payload.NewName+".json")
	if _, err := os.Stat(newPath); err == nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("collection %q already exists", payload.NewName))
		return
	}

	c.Name = payload.NewName
	c.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := atomicWriteJSON(h.collectionsDir, payload.NewName+".json", &c); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("rename failed: %s", err))
		return
	}
	if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
		log.Printf("[daemon %s] warning: failed to remove old collection %s: %v", timeStr(), payload.Name, err)
	}

	h.removeFromIndex("collection", payload.Name)
	h.indexCollection(&c)
	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{
		"name": c.Name,
	})
}

func (h *Hub) handleCollectionsReorder(incoming *incomingMessage) {
	var payload struct {
		Name      string `json:"name"`
		FromIndex int    `json:"fromIndex"`
		ToIndex   int    `json:"toIndex"`
	}
	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid payload")
		return
	}

	if err := validateName(payload.Name); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, err.Error())
		return
	}

	path := filepath.Join(h.collectionsDir, payload.Name+".json")
	var c Collection
	if err := loadJSON(path, &c); err != nil {
		notFound := &ResourceNotFoundError{Kind: "collection", Name: payload.Name, Err: ErrCollectionNotFound}
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(notFound), notFound.Error())
		return
	}

	if payload.FromIndex < 0 || payload.FromIndex >= len(c.Items) ||
		payload.ToIndex < 0 || payload.ToIndex >= len(c.Items) {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "index out of range")
		return
	}

	// Move item from fromIndex to toIndex
	item := c.Items[payload.FromIndex]
	// Remove from old position
	c.Items = append(c.Items[:payload.FromIndex], c.Items[payload.FromIndex+1:]...)
	// Insert at new position
	c.Items = append(c.Items[:payload.ToIndex], append([]CollectionItem{item}, c.Items[payload.ToIndex:]...)...)
	c.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := atomicWriteJSON(h.collectionsDir, payload.Name+".json", &c); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("save failed: %s", err))
		return
	}

	h.indexCollection(&c)
	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{
		"name":      c.Name,
		"itemCount": len(c.Items),
	})
}
