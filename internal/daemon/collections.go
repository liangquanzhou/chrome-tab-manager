package daemon

import (
	"context"
	"encoding/json"
	"fmt"
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
	case "collections.removeItems":
		h.handleCollectionsRemoveItems(incoming)
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

	c.Items = append(c.Items, payload.Items...)
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
