package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"ctm/internal/bookmarks"
	"ctm/internal/protocol"
)

func (h *Hub) handleBookmarksAction(ctx context.Context, incoming *incomingMessage) {
	switch incoming.msg.Action {
	case "bookmarks.tree":
		h.handleBookmarksTree(ctx, incoming)
	case "bookmarks.search":
		h.handleBookmarksSearch(ctx, incoming)
	case "bookmarks.get":
		h.handleBookmarksGet(ctx, incoming)
	case "bookmarks.mirror":
		h.handleBookmarksMirror(ctx, incoming)
	case "bookmarks.overlay.set":
		h.handleBookmarksOverlaySet(incoming)
	case "bookmarks.overlay.get":
		h.handleBookmarksOverlayGet(incoming)
	case "bookmarks.remove":
		h.handleBookmarksRemove(ctx, incoming)
	case "bookmarks.create":
		h.handleBookmarksCreate(ctx, incoming)
	case "bookmarks.update":
		h.handleBookmarksUpdate(ctx, incoming)
	case "bookmarks.export":
		h.handleBookmarksExport(incoming)
	default:
		err := &UnknownActionError{Action: incoming.msg.Action}
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(err), err.Error())
	}
}

// bookmarks.tree: forward to extension, then update local mirror
func (h *Hub) handleBookmarksTree(ctx context.Context, incoming *incomingMessage) {
	target, err := h.resolveTarget(incoming.msg.Target)
	if err != nil {
		sendError(incoming.writer, incoming.msg.ID, targetErrorCode(err), err.Error())
		return
	}

	go func() {
		resp, err := h.sendToExtensionAndWait(ctx, target, "bookmarks.tree", incoming.msg.Payload)
		if err != nil {
			sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(err),
				"failed to get bookmarks tree: "+err.Error())
			return
		}

		// Forward the response to the client
		resp.ID = incoming.msg.ID
		incoming.writer.Write(resp)

		// Update local mirror in background
		h.updateMirrorFromPayload(resp.Payload, target.id)
	}()
}

// bookmarks.search: forward to extension
func (h *Hub) handleBookmarksSearch(ctx context.Context, incoming *incomingMessage) {
	target, err := h.resolveTarget(incoming.msg.Target)
	if err != nil {
		sendError(incoming.writer, incoming.msg.ID, targetErrorCode(err), err.Error())
		return
	}

	// Register pending and forward to extension
	ch := h.registerPendingCh(incoming.msg.ID)

	if err := target.writer.Write(incoming.msg); err != nil {
		h.unregisterPending(incoming.msg.ID)
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(ErrExtensionWriteFail), ErrExtensionWriteFail.Error())
		return
	}

	go func() {
		select {
		case resp := <-ch:
			incoming.writer.Write(resp)
		case <-time.After(10 * time.Second):
			h.unregisterPending(incoming.msg.ID)
			tErr := &ExtensionTimeoutError{Action: incoming.msg.Action}
			sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(tErr), tErr.Error())
		case <-ctx.Done():
			h.unregisterPending(incoming.msg.ID)
		}
	}()
}

// bookmarks.get: forward to extension
func (h *Hub) handleBookmarksGet(ctx context.Context, incoming *incomingMessage) {
	target, err := h.resolveTarget(incoming.msg.Target)
	if err != nil {
		sendError(incoming.writer, incoming.msg.ID, targetErrorCode(err), err.Error())
		return
	}

	ch := h.registerPendingCh(incoming.msg.ID)

	if err := target.writer.Write(incoming.msg); err != nil {
		h.unregisterPending(incoming.msg.ID)
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(ErrExtensionWriteFail), ErrExtensionWriteFail.Error())
		return
	}

	go func() {
		select {
		case resp := <-ch:
			incoming.writer.Write(resp)
		case <-time.After(10 * time.Second):
			h.unregisterPending(incoming.msg.ID)
			tErr := &ExtensionTimeoutError{Action: incoming.msg.Action}
			sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(tErr), tErr.Error())
		case <-ctx.Done():
			h.unregisterPending(incoming.msg.ID)
		}
	}()
}

// bookmarks.mirror: read local mirror, if not available fetch from extension.
// Tries per-target mirror first (mirror_<targetId>.json), then falls back to mirror.json.
func (h *Hub) handleBookmarksMirror(ctx context.Context, incoming *incomingMessage) {
	// Try to resolve target for per-target mirror lookup
	target, targetErr := h.resolveTarget(incoming.msg.Target)

	// Try to load per-target mirror first, then fall back to default
	var mirror bookmarks.BookmarkMirror
	loaded := false
	if target != nil {
		perTargetPath := filepath.Join(h.bookmarksDir, mirrorFilename(target.id))
		if err := loadJSON(perTargetPath, &mirror); err == nil {
			loaded = true
		}
	}
	if !loaded {
		defaultPath := filepath.Join(h.bookmarksDir, "mirror.json")
		if err := loadJSON(defaultPath, &mirror); err == nil {
			loaded = true
		}
	}

	if loaded {
		sendResponse(incoming.writer, incoming.msg.ID, map[string]any{
			"nodeCount":   mirror.NodeCount,
			"folderCount": mirror.FolderCount,
			"mirroredAt":  mirror.MirroredAt,
			"targetId":    mirror.TargetID,
		})
		return
	}

	// No local mirror — fetch from extension
	if targetErr != nil {
		sendError(incoming.writer, incoming.msg.ID, targetErrorCode(targetErr), targetErr.Error())
		return
	}

	// Route through heavy job queue to serialize expensive operations
	h.jobQueue.Submit(func() {
		resp, err := h.sendToExtensionAndWait(ctx, target, "bookmarks.tree", nil)
		if err != nil {
			sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(err),
				"failed to get bookmarks: "+err.Error())
			return
		}

		mirrorResult := h.updateMirrorFromPayload(resp.Payload, target.id)
		if mirrorResult == nil {
			sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
				"failed to parse bookmark tree")
			return
		}

		sendResponse(incoming.writer, incoming.msg.ID, map[string]any{
			"nodeCount":   mirrorResult.NodeCount,
			"folderCount": mirrorResult.FolderCount,
			"mirroredAt":  mirrorResult.MirroredAt,
			"targetId":    mirrorResult.TargetID,
		})
	})
}

// bookmarks.overlay.set: store overlay locally
func (h *Hub) handleBookmarksOverlaySet(incoming *incomingMessage) {
	var payload struct {
		BookmarkID string   `json:"bookmarkId"`
		Tags       []string `json:"tags"`
		Note       string   `json:"note"`
		Alias      string   `json:"alias"`
	}
	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid payload")
		return
	}

	if err := validatePathSafe(payload.BookmarkID); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("invalid bookmarkId: %s", err))
		return
	}

	overlay := &bookmarks.BookmarkOverlay{
		BookmarkID: payload.BookmarkID,
		Tags:       payload.Tags,
		Note:       payload.Note,
		Alias:      payload.Alias,
	}
	if overlay.Tags == nil {
		overlay.Tags = []string{}
	}

	filename := payload.BookmarkID + ".json"
	if err := atomicWriteJSON(h.overlaysDir, filename, overlay); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("save overlay failed: %s", err))
		return
	}

	sendResponse(incoming.writer, incoming.msg.ID, overlay)
}

// bookmarks.overlay.get: read overlay from local storage
func (h *Hub) handleBookmarksOverlayGet(incoming *incomingMessage) {
	var payload struct {
		BookmarkID string `json:"bookmarkId"`
	}
	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid payload")
		return
	}

	if err := validatePathSafe(payload.BookmarkID); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("invalid bookmarkId: %s", err))
		return
	}

	path := filepath.Join(h.overlaysDir, payload.BookmarkID+".json")
	var overlay bookmarks.BookmarkOverlay
	if err := loadJSON(path, &overlay); err != nil {
		// Return empty overlay if not found
		sendResponse(incoming.writer, incoming.msg.ID, &bookmarks.BookmarkOverlay{
			BookmarkID: payload.BookmarkID,
			Tags:       []string{},
		})
		return
	}

	sendResponse(incoming.writer, incoming.msg.ID, &overlay)
}

// bookmarks.export: read mirror and export as markdown
func (h *Hub) handleBookmarksExport(incoming *incomingMessage) {
	var payload struct {
		FolderID string `json:"folderId"`
		Format   string `json:"format"`
		TargetID string `json:"targetId"`
	}
	if incoming.msg.Payload != nil {
		json.Unmarshal(incoming.msg.Payload, &payload)
	}

	// Try per-target mirror first, then fall back to default
	var mirror bookmarks.BookmarkMirror
	loaded := false
	if payload.TargetID != "" {
		perTargetPath := filepath.Join(h.bookmarksDir, mirrorFilename(payload.TargetID))
		if err := loadJSON(perTargetPath, &mirror); err == nil {
			loaded = true
		}
	}
	if !loaded {
		defaultPath := filepath.Join(h.bookmarksDir, "mirror.json")
		if err := loadJSON(defaultPath, &mirror); err != nil {
			sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
				"no bookmark mirror available, run 'bookmarks mirror' first")
			return
		}
	}

	tree := mirror.Tree

	// If folderId is specified, find that subtree
	if payload.FolderID != "" {
		node := bookmarks.FindNode(tree, payload.FolderID)
		if node == nil {
			sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
				fmt.Sprintf("folder %q not found", payload.FolderID))
			return
		}
		if node.Children != nil {
			tree = node.Children
		} else {
			tree = []*bookmarks.BookmarkNode{node}
		}
	}

	content := bookmarks.ExportMarkdown(tree, 0)

	sendResponse(incoming.writer, incoming.msg.ID, map[string]string{
		"content": content,
	})
}

// bookmarks.remove: forward to extension
func (h *Hub) handleBookmarksRemove(ctx context.Context, incoming *incomingMessage) {
	h.forwardToExtension(ctx, incoming)
}

// bookmarks.create: forward to extension
func (h *Hub) handleBookmarksCreate(ctx context.Context, incoming *incomingMessage) {
	h.forwardToExtension(ctx, incoming)
}

// bookmarks.update: forward to extension
func (h *Hub) handleBookmarksUpdate(ctx context.Context, incoming *incomingMessage) {
	h.forwardToExtension(ctx, incoming)
}

// mirrorFilename returns the mirror filename for a given targetID.
// If targetID is empty, returns "mirror.json" (backward compat).
func mirrorFilename(targetID string) string {
	if targetID == "" {
		return "mirror.json"
	}
	return "mirror_" + targetID + ".json"
}

// listMirrorFiles returns all mirror JSON files in the bookmarks directory.
func listMirrorFiles(dir string) ([]string, error) {
	files, err := listJSONFiles(dir)
	if err != nil {
		return nil, err
	}
	var mirrors []string
	for _, f := range files {
		if f == "mirror.json" || strings.HasPrefix(f, "mirror_") {
			mirrors = append(mirrors, f)
		}
	}
	return mirrors, nil
}

// updateMirrorFromPayload parses the extension response and saves a mirror file.
// Writes both a per-target file (mirror_<targetId>.json) and the default mirror.json.
func (h *Hub) updateMirrorFromPayload(payload json.RawMessage, targetID string) *bookmarks.BookmarkMirror {
	var treeData struct {
		Tree []*bookmarks.BookmarkNode `json:"tree"`
	}
	if err := json.Unmarshal(payload, &treeData); err != nil {
		log.Printf("[daemon %s] failed to parse bookmark tree for mirror: %v", timeStr(), err)
		return nil
	}

	nodeCount, folderCount := bookmarks.CountNodes(treeData.Tree)
	mirror := &bookmarks.BookmarkMirror{
		Tree:        treeData.Tree,
		MirroredAt:  time.Now().UTC().Format(time.RFC3339),
		TargetID:    targetID,
		NodeCount:   nodeCount,
		FolderCount: folderCount,
	}

	// Always write default mirror.json (backward compat)
	if err := atomicWriteJSON(h.bookmarksDir, "mirror.json", mirror); err != nil {
		log.Printf("[daemon %s] failed to save bookmark mirror: %v", timeStr(), err)
		return nil
	}

	// Also write per-target mirror file if targetID is provided
	if targetID != "" {
		perTargetFile := mirrorFilename(targetID)
		if err := atomicWriteJSON(h.bookmarksDir, perTargetFile, mirror); err != nil {
			log.Printf("[daemon %s] failed to save per-target bookmark mirror %s: %v",
				timeStr(), perTargetFile, err)
			// Don't fail — the default mirror was written successfully
		}
	}

	log.Printf("[daemon %s] bookmark mirror updated: %d nodes, %d folders (target=%s)",
		timeStr(), nodeCount, folderCount, targetID)
	return mirror
}
