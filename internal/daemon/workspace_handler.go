package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"ctm/internal/protocol"
	"ctm/internal/workspace"
)

// workspaceIDRe matches valid workspace IDs: letters, digits, underscore, hyphen.
// Workspace IDs are generated as "ws_<timestamp>_<counter>" but we allow any
// safe character combination to be forward-compatible.
var workspaceIDRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// validateWorkspaceID checks that a workspace ID is safe for use in file paths.
// It rejects empty strings, path separators, ".." sequences, and any character
// outside the allowed set.
func validateWorkspaceID(id string) error {
	if id == "" {
		return fmt.Errorf("id cannot be empty")
	}
	if len(id) > 128 {
		return fmt.Errorf("id too long (max 128 characters)")
	}
	if !workspaceIDRe.MatchString(id) {
		return fmt.Errorf("id contains invalid characters (allowed: a-z A-Z 0-9 _ -)")
	}
	return nil
}

func (h *Hub) handleWorkspaceAction(ctx context.Context, incoming *incomingMessage) {
	switch incoming.msg.Action {
	case "workspace.list":
		h.handleWorkspaceList(incoming)
	case "workspace.get":
		h.handleWorkspaceGet(incoming)
	case "workspace.create":
		h.handleWorkspaceCreate(incoming)
	case "workspace.update":
		h.handleWorkspaceUpdate(incoming)
	case "workspace.delete":
		h.handleWorkspaceDelete(incoming)
	case "workspace.switch":
		h.handleWorkspaceSwitch(ctx, incoming)
	default:
		err := &UnknownActionError{Action: incoming.msg.Action}
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(err), err.Error())
	}
}

func (h *Hub) handleWorkspaceList(incoming *incomingMessage) {
	files, err := listJSONFiles(h.workspacesDir)
	if err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("list workspaces: %s", err))
		return
	}

	summaries := make([]workspace.WorkspaceSummary, 0, len(files))
	for _, f := range files {
		var w workspace.Workspace
		if err := loadJSON(filepath.Join(h.workspacesDir, f), &w); err != nil {
			continue
		}
		summaries = append(summaries, w.Summary())
	}

	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{"workspaces": summaries})
}

func (h *Hub) handleWorkspaceGet(incoming *incomingMessage) {
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid payload")
		return
	}

	if err := validateWorkspaceID(payload.ID); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, err.Error())
		return
	}

	path := filepath.Join(h.workspacesDir, payload.ID+".json")
	var w workspace.Workspace
	if err := loadJSON(path, &w); err != nil {
		notFound := &ResourceNotFoundError{Kind: "workspace", Name: payload.ID, Err: ErrWorkspaceNotFound}
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(notFound), notFound.Error())
		return
	}

	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{"workspace": w})
}

func (h *Hub) handleWorkspaceCreate(incoming *incomingMessage) {
	var payload struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Sessions    []string `json:"sessions"`
		Collections []string `json:"collections"`
		Tags        []string `json:"tags"`
	}
	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid payload")
		return
	}

	if payload.Name == "" {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "name cannot be empty")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	w := &workspace.Workspace{
		ID:          workspace.GenerateID(),
		Name:        payload.Name,
		Description: payload.Description,
		Sessions:    ensureStringSlice(payload.Sessions),
		Collections: ensureStringSlice(payload.Collections),
		Tags:        ensureStringSlice(payload.Tags),
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	filename := w.ID + ".json"
	if err := atomicWriteJSON(h.workspacesDir, filename, w); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("create failed: %s", err))
		return
	}

	h.indexWorkspace(w.ID, w.Name, w.Tags)
	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{
		"id":   w.ID,
		"name": w.Name,
	})
}

func (h *Hub) handleWorkspaceUpdate(incoming *incomingMessage) {
	var payload struct {
		ID                string   `json:"id"`
		Name              *string  `json:"name,omitempty"`
		Description       *string  `json:"description,omitempty"`
		Sessions          []string `json:"sessions,omitempty"`
		Collections       []string `json:"collections,omitempty"`
		BookmarkFolderIDs []string `json:"bookmarkFolderIds,omitempty"`
		SavedSearchIDs    []string `json:"savedSearchIds,omitempty"`
		Tags              []string `json:"tags,omitempty"`
		Notes             *string  `json:"notes,omitempty"`
		Status            *string  `json:"status,omitempty"`
		DefaultTarget     *string  `json:"defaultTarget,omitempty"`
	}
	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid payload")
		return
	}

	if err := validateWorkspaceID(payload.ID); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, err.Error())
		return
	}

	path := filepath.Join(h.workspacesDir, payload.ID+".json")
	var w workspace.Workspace
	if err := loadJSON(path, &w); err != nil {
		notFound := &ResourceNotFoundError{Kind: "workspace", Name: payload.ID, Err: ErrWorkspaceNotFound}
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(notFound), notFound.Error())
		return
	}

	// Apply updates
	if payload.Name != nil {
		w.Name = *payload.Name
	}
	if payload.Description != nil {
		w.Description = *payload.Description
	}
	if payload.Sessions != nil {
		w.Sessions = payload.Sessions
	}
	if payload.Collections != nil {
		w.Collections = payload.Collections
	}
	if payload.BookmarkFolderIDs != nil {
		w.BookmarkFolderIDs = payload.BookmarkFolderIDs
	}
	if payload.SavedSearchIDs != nil {
		w.SavedSearchIDs = payload.SavedSearchIDs
	}
	if payload.Tags != nil {
		w.Tags = payload.Tags
	}
	if payload.Notes != nil {
		w.Notes = *payload.Notes
	}
	if payload.Status != nil {
		w.Status = *payload.Status
	}
	if payload.DefaultTarget != nil {
		w.DefaultTarget = *payload.DefaultTarget
	}

	w.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := atomicWriteJSON(h.workspacesDir, payload.ID+".json", &w); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("update failed: %s", err))
		return
	}

	h.indexWorkspace(w.ID, w.Name, w.Tags)
	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{})
}

func (h *Hub) handleWorkspaceDelete(incoming *incomingMessage) {
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid payload")
		return
	}

	if err := validateWorkspaceID(payload.ID); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, err.Error())
		return
	}

	path := filepath.Join(h.workspacesDir, payload.ID+".json")
	if err := os.Remove(path); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("delete failed: %s", err))
		return
	}

	h.removeFromIndex("workspace", payload.ID)
	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{})
}

func (h *Hub) handleWorkspaceSwitch(ctx context.Context, incoming *incomingMessage) {
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid payload")
		return
	}

	if err := validateWorkspaceID(payload.ID); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, err.Error())
		return
	}

	// Load workspace
	path := filepath.Join(h.workspacesDir, payload.ID+".json")
	var w workspace.Workspace
	if err := loadJSON(path, &w); err != nil {
		notFound := &ResourceNotFoundError{Kind: "workspace", Name: payload.ID, Err: ErrWorkspaceNotFound}
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(notFound), notFound.Error())
		return
	}

	target, err := h.resolveTarget(incoming.msg.Target)
	if err != nil {
		sendError(incoming.writer, incoming.msg.ID, targetErrorCode(err), err.Error())
		return
	}

	// Route through heavy job queue to serialize expensive operations
	h.jobQueue.Submit(func() {
		tabsClosed := 0
		windowsCreated := 0
		tabsOpened := 0
		tabsFailed := 0

		// Step 1: Close all current tabs
		tabsResp, err := h.sendToExtensionAndWait(ctx, target, "tabs.list", nil)
		if err == nil {
			var tabsData struct {
				Tabs []struct {
					ID int `json:"id"`
				} `json:"tabs"`
			}
			if json.Unmarshal(tabsResp.Payload, &tabsData) == nil {
				for _, tab := range tabsData.Tabs {
					_, err := h.sendToExtensionAndWait(ctx, target, "tabs.close",
						mustMarshal(map[string]any{"tabId": tab.ID}))
					if err == nil {
						tabsClosed++
					}
				}
			}
		}

		// Step 2: Restore the first session in the workspace
		if len(w.Sessions) > 0 {
			sessionName := w.Sessions[0]
			sessionPath := filepath.Join(h.sessionsDir, sessionName+".json")
			var s Session
			if err := loadJSON(sessionPath, &s); err == nil {
				// Collect all tabs across windows for throttled batch opening
				type tabWithMeta struct {
					tab       SessionTab
					windowIdx int
				}
				var allTabs []tabWithMeta
				windowSet := make(map[int]bool)
				for wi, win := range s.Windows {
					if len(win.Tabs) == 0 {
						continue
					}
					windowSet[wi] = true
					for _, t := range win.Tabs {
						allTabs = append(allTabs, tabWithMeta{tab: t, windowIdx: wi})
					}
				}
				windowsCreated = len(windowSet)

				groupTabIDs := make(map[int][]int)

				// Throttled batch: open tabs in batches of 5 with 200ms delay
				opened, failed := throttledBatch(ctx, allTabs, restoreBatchSize, restoreBatchDelay, func(item tabWithMeta) error {
					resp, err := h.sendToExtensionAndWait(ctx, target, "tabs.open",
						mustMarshal(map[string]any{
							"url":    item.tab.URL,
							"active": item.tab.Active,
						}))
					if err != nil {
						return err
					}
					if item.tab.GroupIndex >= 0 {
						var tabResult struct {
							TabID int `json:"tabId"`
						}
						json.Unmarshal(resp.Payload, &tabResult)
						if tabResult.TabID > 0 {
							groupTabIDs[item.tab.GroupIndex] = append(groupTabIDs[item.tab.GroupIndex], tabResult.TabID)
						}
					}
					return nil
				})
				tabsOpened = opened
				tabsFailed = failed

				// Create groups
				for idx, g := range s.Groups {
					tabIDs, ok := groupTabIDs[idx]
					if !ok || len(tabIDs) == 0 {
						continue
					}
					h.sendToExtensionAndWait(ctx, target, "groups.create",
						mustMarshal(map[string]any{
							"tabIds": tabIDs,
							"title":  g.Title,
							"color":  g.Color,
						}))
				}
			}
		}

		// Update workspace lastActiveAt
		w.LastActiveAt = time.Now().UTC().Format(time.RFC3339)
		w.UpdatedAt = w.LastActiveAt
		atomicWriteJSON(h.workspacesDir, payload.ID+".json", &w)
		h.indexWorkspace(w.ID, w.Name, w.Tags)

		sendResponse(incoming.writer, incoming.msg.ID, map[string]any{
			"tabsClosed":     tabsClosed,
			"windowsCreated": windowsCreated,
			"tabsOpened":     tabsOpened,
			"tabsFailed":     tabsFailed,
		})
	})
}

// ensureStringSlice returns an empty slice instead of nil.
func ensureStringSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
