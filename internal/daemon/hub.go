package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"ctm/internal/protocol"
	"ctm/internal/search"
)

type Hub struct {
	// Mutable state — only accessed in Run() goroutine
	targets       map[string]*targetConn
	connections   map[net.Conn]*connState
	subscribers   []*subscriber
	defaultTarget string
	targetCounter int

	// Pending requests — protected by its own mutex (accessed from handler goroutines)
	pendingMu sync.Mutex
	pending   map[string]chan *protocol.Message

	// Channels for communication with Run()
	registerCh   chan *connRegistration
	unregisterCh chan net.Conn
	messageCh    chan *incomingMessage
	stopCh       chan struct{}

	// Heavy job queue — serializes expensive operations (restore, switch, mirror)
	jobQueue *JobQueue

	// Search index — supplementary index for faster search
	index *search.SearchIndex

	// Storage directories
	sessionsDir      string
	collectionsDir   string
	bookmarksDir     string
	overlaysDir      string
	workspacesDir    string
	savedSearchesDir string
	syncCloudDir     string
}

type targetConn struct {
	id           string
	conn         net.Conn
	writer       *protocol.Writer
	channel      string
	label        string
	userAgent    string
	capabilities []string
	connectedAt  time.Time
}

type connState struct {
	conn     net.Conn
	writer   *protocol.Writer
	isTarget bool
	targetID string
}

type subscriber struct {
	conn     net.Conn
	writer   *protocol.Writer
	patterns []string
}

type connRegistration struct {
	conn   net.Conn
	writer *protocol.Writer
}

type incomingMessage struct {
	conn   net.Conn
	writer *protocol.Writer
	msg    *protocol.Message
}

func NewHub(sessionsDir, collectionsDir, bookmarksDir, overlaysDir, workspacesDir, savedSearchesDir, syncCloudDir, searchIndexPath string) *Hub {
	idx := search.NewSearchIndex(searchIndexPath)
	// Best-effort load; if it fails we start with an empty index
	if err := idx.Load(); err != nil {
		log.Printf("[daemon %s] warning: load search index: %v", timeStr(), err)
	}
	return &Hub{
		targets:          make(map[string]*targetConn),
		connections:      make(map[net.Conn]*connState),
		pending:          make(map[string]chan *protocol.Message),
		registerCh:       make(chan *connRegistration, 16),
		unregisterCh:     make(chan net.Conn, 16),
		messageCh:        make(chan *incomingMessage, 64),
		stopCh:           make(chan struct{}),
		jobQueue:         NewJobQueue(),
		index:            idx,
		sessionsDir:      sessionsDir,
		collectionsDir:   collectionsDir,
		bookmarksDir:     bookmarksDir,
		overlaysDir:      overlaysDir,
		workspacesDir:    workspacesDir,
		savedSearchesDir: savedSearchesDir,
		syncCloudDir:     syncCloudDir,
	}
}

func (h *Hub) Run(ctx context.Context) {
	// Start the heavy job queue in background
	go h.jobQueue.Run(ctx)

	for {
		select {
		case reg := <-h.registerCh:
			h.connections[reg.conn] = &connState{
				conn:   reg.conn,
				writer: reg.writer,
			}

		case conn := <-h.unregisterCh:
			cs, ok := h.connections[conn]
			if !ok {
				continue
			}
			if cs.isTarget {
				delete(h.targets, cs.targetID)
				if h.defaultTarget == cs.targetID {
					h.defaultTarget = ""
				}
				log.Printf("[daemon %s] target %s disconnected", timeStr(), cs.targetID)
			}
			h.removeSubscriber(conn)
			delete(h.connections, conn)
			conn.Close()

		case incoming := <-h.messageCh:
			h.handleMessage(ctx, incoming)

		case <-h.stopCh:
			return

		case <-ctx.Done():
			return
		}
	}
}

func (h *Hub) Stop() {
	select {
	case h.stopCh <- struct{}{}:
	default:
	}
}

func (h *Hub) handleMessage(ctx context.Context, incoming *incomingMessage) {
	msg := incoming.msg

	switch msg.Type {
	case protocol.TypeHello:
		h.handleHello(incoming)
	case protocol.TypeRequest:
		h.handleRequest(ctx, incoming)
	case protocol.TypeResponse, protocol.TypeError:
		h.handlePendingResponse(msg)
	case protocol.TypeEvent:
		h.fanoutEvent(incoming)
	}
}

func (h *Hub) handleHello(incoming *incomingMessage) {
	var payload struct {
		Channel      string   `json:"channel"`
		ExtensionID  string   `json:"extensionId"`
		InstanceID   string   `json:"instanceId"`
		UserAgent    string   `json:"userAgent"`
		Capabilities []string `json:"capabilities"`
		MinSupported int      `json:"min_supported"`
	}

	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid hello payload")
		return
	}

	h.targetCounter++
	targetID := fmt.Sprintf("target_%d", h.targetCounter)

	tc := &targetConn{
		id:           targetID,
		conn:         incoming.conn,
		writer:       incoming.writer,
		channel:      payload.Channel,
		userAgent:    payload.UserAgent,
		capabilities: payload.Capabilities,
		connectedAt:  time.Now(),
	}

	h.targets[targetID] = tc

	if cs := h.connections[incoming.conn]; cs != nil {
		cs.isTarget = true
		cs.targetID = targetID
	}

	// Auto-set default if first target
	if len(h.targets) == 1 {
		h.defaultTarget = targetID
	}

	sendResponse(incoming.writer, incoming.msg.ID, map[string]string{"targetId": targetID})
	log.Printf("[daemon %s] target %s registered (channel=%s)", timeStr(), targetID, payload.Channel)
}

func (h *Hub) handleRequest(ctx context.Context, incoming *incomingMessage) {
	action := incoming.msg.Action

	switch {
	case action == "daemon.stop":
		h.handleDaemonStop(incoming)
	case action == "subscribe":
		h.handleSubscribe(incoming)
	case strings.HasPrefix(action, "targets."):
		h.handleTargetsAction(incoming)
	case strings.HasPrefix(action, "sessions."):
		h.handleSessionsAction(ctx, incoming)
	case strings.HasPrefix(action, "collections."):
		h.handleCollectionsAction(ctx, incoming)
	case strings.HasPrefix(action, "bookmarks."):
		h.handleBookmarksAction(ctx, incoming)
	case strings.HasPrefix(action, "search."):
		h.handleSearchAction(ctx, incoming)
	case strings.HasPrefix(action, "workspace."):
		h.handleWorkspaceAction(ctx, incoming)
	case strings.HasPrefix(action, "sync."):
		h.handleSyncAction(incoming)
	case strings.HasPrefix(action, "tabs.") || strings.HasPrefix(action, "groups.") ||
		strings.HasPrefix(action, "windows.") || strings.HasPrefix(action, "history.") ||
		strings.HasPrefix(action, "downloads."):
		h.forwardToExtension(ctx, incoming)
	default:
		err := &UnknownActionError{Action: action}
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(err), err.Error())
	}
}

func (h *Hub) handleDaemonStop(incoming *incomingMessage) {
	sendResponse(incoming.writer, incoming.msg.ID, map[string]bool{"stopping": true})
	h.Stop()
}

func (h *Hub) handleSubscribe(incoming *incomingMessage) {
	var payload struct {
		Patterns []string `json:"patterns"`
	}
	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid subscribe payload")
		return
	}

	h.subscribers = append(h.subscribers, &subscriber{
		conn:     incoming.conn,
		writer:   incoming.writer,
		patterns: payload.Patterns,
	})

	sendResponse(incoming.writer, incoming.msg.ID, map[string]bool{"subscribed": true})
}

// --- Target actions ---

func (h *Hub) handleTargetsAction(incoming *incomingMessage) {
	switch incoming.msg.Action {
	case "targets.list":
		h.handleTargetsList(incoming)
	case "targets.default":
		h.handleTargetsDefault(incoming)
	case "targets.clearDefault":
		h.handleTargetsClearDefault(incoming)
	case "targets.label":
		h.handleTargetsLabel(incoming)
	default:
		err := &UnknownActionError{Action: incoming.msg.Action}
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(err), err.Error())
	}
}

func (h *Hub) handleTargetsList(incoming *incomingMessage) {
	type targetInfo struct {
		TargetID    string `json:"targetId"`
		Channel     string `json:"channel"`
		Label       string `json:"label"`
		IsDefault   bool   `json:"isDefault"`
		UserAgent   string `json:"userAgent"`
		ConnectedAt int64  `json:"connectedAt"`
	}

	targets := make([]targetInfo, 0, len(h.targets))
	for _, tc := range h.targets {
		targets = append(targets, targetInfo{
			TargetID:    tc.id,
			Channel:     tc.channel,
			Label:       tc.label,
			IsDefault:   tc.id == h.defaultTarget,
			UserAgent:   tc.userAgent,
			ConnectedAt: tc.connectedAt.UnixMilli(),
		})
	}

	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{"targets": targets})
}

func (h *Hub) handleTargetsDefault(incoming *incomingMessage) {
	var payload struct {
		TargetID string `json:"targetId"`
	}
	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid payload")
		return
	}

	if _, ok := h.targets[payload.TargetID]; !ok {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrTargetOffline, "target not found")
		return
	}

	h.defaultTarget = payload.TargetID
	sendResponse(incoming.writer, incoming.msg.ID, map[string]string{"targetId": payload.TargetID})
}

func (h *Hub) handleTargetsClearDefault(incoming *incomingMessage) {
	h.defaultTarget = ""
	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{})
}

func (h *Hub) handleTargetsLabel(incoming *incomingMessage) {
	var payload struct {
		TargetID string `json:"targetId"`
		Label    string `json:"label"`
	}
	if err := json.Unmarshal(incoming.msg.Payload, &payload); err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "invalid payload")
		return
	}

	// Validate label: max 256 chars, no control characters
	if len([]rune(payload.Label)) > 256 {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "label too long (max 256)")
		return
	}
	if strings.ContainsAny(payload.Label, "\x00\n\r") {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload, "label contains invalid characters")
		return
	}

	tc, ok := h.targets[payload.TargetID]
	if !ok {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrTargetOffline, "target not found")
		return
	}

	tc.label = payload.Label
	sendResponse(incoming.writer, incoming.msg.ID, map[string]any{
		"targetId": payload.TargetID,
		"label":    payload.Label,
	})
}

// --- Extension forwarding ---

func (h *Hub) forwardToExtension(ctx context.Context, incoming *incomingMessage) {
	target, err := h.resolveTarget(incoming.msg.Target)
	if err != nil {
		sendError(incoming.writer, incoming.msg.ID, targetErrorCode(err), err.Error())
		return
	}

	// Register pending: when extension responds, forward to client
	ch := h.registerPendingCh(incoming.msg.ID)

	// Forward to extension
	if err := target.writer.Write(incoming.msg); err != nil {
		h.unregisterPending(incoming.msg.ID)
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(ErrExtensionWriteFail), ErrExtensionWriteFail.Error())
		return
	}

	// Wait in goroutine to not block Hub
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

func (h *Hub) resolveTarget(selector *protocol.TargetSelector) (*targetConn, error) {
	if selector != nil && selector.TargetID != "" {
		tc, ok := h.targets[selector.TargetID]
		if !ok {
			return nil, &TargetNotFoundError{TargetID: selector.TargetID}
		}
		return tc, nil
	}

	if h.defaultTarget != "" {
		if tc, ok := h.targets[h.defaultTarget]; ok {
			return tc, nil
		}
	}

	if len(h.targets) == 1 {
		for _, tc := range h.targets {
			return tc, nil
		}
	}

	if len(h.targets) == 0 {
		return nil, ErrNoTargetsConnected
	}

	return nil, ErrMultipleTargetsAmbig
}

func (h *Hub) sendToExtensionAndWait(ctx context.Context, target *targetConn, action string, payload json.RawMessage) (*protocol.Message, error) {
	id := protocol.MakeID()
	ch := h.registerPendingCh(id)

	msg := &protocol.Message{
		ID:              id,
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeRequest,
		Action:          action,
		Payload:         payload,
	}

	if err := target.writer.Write(msg); err != nil {
		h.unregisterPending(id)
		return nil, fmt.Errorf("write to extension: %w", ErrExtensionWriteFail)
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(10 * time.Second):
		h.unregisterPending(id)
		return nil, &ExtensionTimeoutError{Action: action}
	case <-ctx.Done():
		h.unregisterPending(id)
		return nil, ctx.Err()
	}
}

// focusBrowserWindow asks the extension to focus the last-focused Chrome window.
// Best-effort: errors are silently ignored.
func (h *Hub) focusBrowserWindow(ctx context.Context, target *targetConn) {
	h.sendToExtensionAndWait(ctx, target, "windows.focus", nil)
}

// --- Event fanout ---

func (h *Hub) fanoutEvent(incoming *incomingMessage) {
	for _, sub := range h.subscribers {
		if matchesPatterns(incoming.msg.Action, sub.patterns) {
			sub.writer.Write(incoming.msg)
		}
	}
}

func (h *Hub) removeSubscriber(conn net.Conn) {
	filtered := h.subscribers[:0]
	for _, sub := range h.subscribers {
		if sub.conn != conn {
			filtered = append(filtered, sub)
		}
	}
	h.subscribers = filtered
}

// --- Pending request management ---

func (h *Hub) registerPendingCh(id string) chan *protocol.Message {
	ch := make(chan *protocol.Message, 1)
	h.pendingMu.Lock()
	h.pending[id] = ch
	h.pendingMu.Unlock()
	return ch
}

func (h *Hub) unregisterPending(id string) {
	h.pendingMu.Lock()
	delete(h.pending, id)
	h.pendingMu.Unlock()
}

func (h *Hub) handlePendingResponse(msg *protocol.Message) {
	h.pendingMu.Lock()
	ch, ok := h.pending[msg.ID]
	if ok {
		delete(h.pending, msg.ID)
	}
	h.pendingMu.Unlock()

	if ok {
		ch <- msg
	}
}

// --- Pattern matching ---

func matchesPatterns(action string, patterns []string) bool {
	for _, p := range patterns {
		if matchPattern(action, p) {
			return true
		}
	}
	return false
}

func matchPattern(action, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		return strings.HasPrefix(action, prefix+".")
	}
	return action == pattern
}

// --- Helpers ---

func sendError(w *protocol.Writer, id string, code protocol.ErrorCode, message string) {
	msg := &protocol.Message{
		ID:              id,
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeError,
		Error: &protocol.ErrorBody{
			Code:    code,
			Message: message,
		},
	}
	w.Write(msg)
}

func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}


// --- Search index helpers ---

// indexSession upserts a session into the search index.
func (h *Hub) indexSession(s *Session) {
	h.index.Upsert(&search.IndexEntry{
		Kind:      "session",
		ID:        s.Name,
		Title:     s.Name,
		UpdatedAt: time.Now(),
		Checksum:  search.Checksum(mustMarshal(s)),
	})
	h.saveIndex()
}

// indexCollection upserts a collection into the search index.
func (h *Hub) indexCollection(c *Collection) {
	h.index.Upsert(&search.IndexEntry{
		Kind:      "collection",
		ID:        c.Name,
		Title:     c.Name,
		UpdatedAt: time.Now(),
		Checksum:  search.Checksum(mustMarshal(c)),
	})
	h.saveIndex()
}

// indexWorkspace upserts a workspace into the search index.
func (h *Hub) indexWorkspace(id, name string, tags []string) {
	h.index.Upsert(&search.IndexEntry{
		Kind:      "workspace",
		ID:        id,
		Title:     name,
		Tags:      tags,
		UpdatedAt: time.Now(),
	})
	h.saveIndex()
}

// removeFromIndex removes an entry from the search index.
func (h *Hub) removeFromIndex(kind, id string) {
	h.index.Remove(kind, id)
	h.saveIndex()
}

// saveIndex persists the search index to disk (best-effort).
func (h *Hub) saveIndex() {
	if err := h.index.Save(); err != nil {
		log.Printf("[daemon %s] warning: save search index: %v", timeStr(), err)
	}
}

func timeStr() string {
	return time.Now().Format("15:04:05")
}
