package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ctm/internal/protocol"
)

type SessionTab struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	Pinned     bool   `json:"pinned"`
	Active     bool   `json:"active"`
	GroupIndex int    `json:"groupIndex"`
}

type SessionWindow struct {
	Tabs []SessionTab `json:"tabs"`
}

type SessionGroup struct {
	Title     string `json:"title"`
	Color     string `json:"color"`
	Collapsed bool   `json:"collapsed"`
}

type Session struct {
	Name         string          `json:"name"`
	CreatedAt    string          `json:"createdAt"`
	SourceTarget string          `json:"sourceTarget"`
	Windows      []SessionWindow `json:"windows"`
	Groups       []SessionGroup  `json:"groups"`
}

type SessionSummary struct {
	Name         string `json:"name"`
	TabCount     int    `json:"tabCount"`
	WindowCount  int    `json:"windowCount"`
	GroupCount   int    `json:"groupCount"`
	CreatedAt    string `json:"createdAt"`
	SourceTarget string `json:"sourceTarget"`
}

func (s *Session) Summary() SessionSummary {
	tabCount := 0
	for _, w := range s.Windows {
		tabCount += len(w.Tabs)
	}
	return SessionSummary{
		Name:         s.Name,
		TabCount:     tabCount,
		WindowCount:  len(s.Windows),
		GroupCount:   len(s.Groups),
		CreatedAt:    s.CreatedAt,
		SourceTarget: s.SourceTarget,
	}
}

func (h *Hub) handleSessionsAction(ctx context.Context, incoming *incomingMessage) {
	switch incoming.msg.Action {
	case "sessions.list":
		h.handleSessionsList(incoming)
	case "sessions.get":
		h.handleSessionsGet(incoming)
	case "sessions.save":
		h.handleSessionsSave(ctx, incoming)
	case "sessions.restore":
		h.handleSessionsRestore(ctx, incoming)
	case "sessions.delete":
		h.handleSessionsDelete(incoming)
	default:
		err := &UnknownActionError{Action: incoming.msg.Action}
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(err), err.Error())
	}
}

func (h *Hub) handleSessionsList(incoming *incomingMessage) {
	files, err := listJSONFiles(h.sessionsDir)
	if err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("list sessions: %s", err))
		return
	}

	summaries := make([]SessionSummary, 0, len(files))
	for _, f := range files {
		var s Session
		if err := loadJSON(filepath.Join(h.sessionsDir, f), &s); err != nil {
			continue
		}
		summaries = append(summaries, s.Summary())
	}

	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{"sessions": summaries})
}

func (h *Hub) handleSessionsGet(incoming *incomingMessage) {
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

	path := filepath.Join(h.sessionsDir, payload.Name+".json")
	var s Session
	if err := loadJSON(path, &s); err != nil {
		notFound := &ResourceNotFoundError{Kind: "session", Name: payload.Name, Err: ErrSessionNotFound}
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(notFound), notFound.Error())
		return
	}

	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{"session": s})
}

func (h *Hub) handleSessionsSave(ctx context.Context, incoming *incomingMessage) {
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

	target, err := h.resolveTarget(incoming.msg.Target)
	if err != nil {
		sendError(incoming.writer, incoming.msg.ID, targetErrorCode(err), err.Error())
		return
	}

	// Run in goroutine: queries extension then saves file
	go func() {
		tabsResp, err := h.sendToExtensionAndWait(ctx, target, "tabs.list", nil)
		if err != nil {
			sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(err),
				"failed to get tabs: "+err.Error())
			return
		}

		groupsResp, err := h.sendToExtensionAndWait(ctx, target, "groups.list", nil)
		if err != nil {
			sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(err),
				"failed to get groups: "+err.Error())
			return
		}

		session := buildSession(payload.Name, target.id, tabsResp.Payload, groupsResp.Payload)

		if err := atomicWriteJSON(h.sessionsDir, payload.Name+".json", session); err != nil {
			sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
				"failed to save: "+err.Error())
			return
		}

		h.indexSession(session)
		sendResponse(incoming.writer, incoming.msg.ID, session.Summary())
	}()
}

func (h *Hub) handleSessionsRestore(ctx context.Context, incoming *incomingMessage) {
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

	path := filepath.Join(h.sessionsDir, payload.Name+".json")
	var s Session
	if err := loadJSON(path, &s); err != nil {
		notFound := &ResourceNotFoundError{Kind: "session", Name: payload.Name, Err: ErrSessionNotFound}
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
		windowsCreated := 0
		groupsCreated := 0

		// Lesson 3: windows → tabs → groups (strict order)
		// Collect all tabs across windows for throttled batch opening
		type tabWithMeta struct {
			tab        SessionTab
			windowIdx  int
		}
		var allTabs []tabWithMeta
		windowSet := make(map[int]bool)
		for wi, w := range s.Windows {
			if len(w.Tabs) == 0 {
				continue
			}
			windowSet[wi] = true
			for _, t := range w.Tabs {
				allTabs = append(allTabs, tabWithMeta{tab: t, windowIdx: wi})
			}
		}
		windowsCreated = len(windowSet)

		// Track tab IDs per group index for group creation
		groupTabIDs := make(map[int][]int)

		// Throttled batch: open tabs in batches of 5 with 200ms delay
		tabsOpened, tabsFailed := throttledBatch(ctx, allTabs, restoreBatchSize, restoreBatchDelay, func(item tabWithMeta) error {
			resp, err := h.sendToExtensionAndWait(ctx, target, "tabs.open", mustMarshal(map[string]any{
				"url":    item.tab.URL,
				"active": item.tab.Active,
			}))
			if err != nil {
				return err
			}
			// Track tab ID for group assignment
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

		// Create groups after all tabs are in place
		for idx, g := range s.Groups {
			tabIDs, ok := groupTabIDs[idx]
			if !ok || len(tabIDs) == 0 {
				continue
			}
			_, err := h.sendToExtensionAndWait(ctx, target, "groups.create", mustMarshal(map[string]any{
				"tabIds": tabIDs,
				"title":  g.Title,
				"color":  g.Color,
			}))
			if err == nil {
				groupsCreated++
			}
		}

		// Focus the browser window so user can see what was restored
		h.focusBrowserWindow(ctx, target)

		sendResponse(incoming.writer, incoming.msg.ID, map[string]any{
			"windowsCreated": windowsCreated,
			"tabsOpened":     tabsOpened,
			"tabsFailed":     tabsFailed,
			"groupsCreated":  groupsCreated,
		})
	})
}

func (h *Hub) handleSessionsDelete(incoming *incomingMessage) {
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

	path := filepath.Join(h.sessionsDir, payload.Name+".json")
	if err := os.Remove(path); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("delete failed: %s", err))
		return
	}

	h.removeFromIndex("session", payload.Name)
	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{})
}

func buildSession(name, targetID string, tabsPayload, groupsPayload json.RawMessage) *Session {
	type rawTab struct {
		ID       int    `json:"id"`
		WindowID int    `json:"windowId"`
		Index    int    `json:"index"`
		Title    string `json:"title"`
		URL      string `json:"url"`
		Active   bool   `json:"active"`
		Pinned   bool   `json:"pinned"`
		GroupID  int    `json:"groupId"`
	}
	type rawGroup struct {
		ID        int    `json:"id"`
		Title     string `json:"title"`
		Color     string `json:"color"`
		Collapsed bool   `json:"collapsed"`
	}

	var tabsData struct {
		Tabs []rawTab `json:"tabs"`
	}
	json.Unmarshal(tabsPayload, &tabsData)

	var groupsData struct {
		Groups []rawGroup `json:"groups"`
	}
	json.Unmarshal(groupsPayload, &groupsData)

	// Build group index map: chrome groupID → session group index
	groupIndex := make(map[int]int)
	var sessionGroups []SessionGroup
	for i, g := range groupsData.Groups {
		groupIndex[g.ID] = i
		sessionGroups = append(sessionGroups, SessionGroup{
			Title:     g.Title,
			Color:     g.Color,
			Collapsed: g.Collapsed,
		})
	}

	// Group tabs by windowId
	windowTabs := make(map[int][]rawTab)
	var windowIDs []int
	for _, t := range tabsData.Tabs {
		if _, seen := windowTabs[t.WindowID]; !seen {
			windowIDs = append(windowIDs, t.WindowID)
		}
		windowTabs[t.WindowID] = append(windowTabs[t.WindowID], t)
	}

	var sessionWindows []SessionWindow
	for _, wid := range windowIDs {
		tabs := windowTabs[wid]
		var sessionTabs []SessionTab
		for _, t := range tabs {
			gIdx := -1
			if t.GroupID > 0 {
				if idx, ok := groupIndex[t.GroupID]; ok {
					gIdx = idx
				}
			}
			sessionTabs = append(sessionTabs, SessionTab{
				URL:        t.URL,
				Title:      t.Title,
				Pinned:     t.Pinned,
				Active:     t.Active,
				GroupIndex: gIdx,
			})
		}
		sessionWindows = append(sessionWindows, SessionWindow{Tabs: sessionTabs})
	}

	return &Session{
		Name:         name,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		SourceTarget: targetID,
		Windows:      sessionWindows,
		Groups:       sessionGroups,
	}
}

func sendResponse(w *protocol.Writer, id string, payload any) {
	resp := &protocol.Message{
		ID:              id,
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeResponse,
		Payload:         mustMarshal(payload),
	}
	w.Write(resp)
}

func sessionNameFromFile(filename string) string {
	return strings.TrimSuffix(filename, ".json")
}
