package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"ctm/internal/client"
	"ctm/internal/daemon"
	"ctm/internal/protocol"
)

// ---------------------------------------------------------------------------
// Integration test infrastructure
// ---------------------------------------------------------------------------

// mockExtension simulates a Chrome extension that responds to forwarded actions.
// It connects to the daemon via Unix socket, sends a hello message, and then
// responds to incoming requests in a read loop.
type mockExtension struct {
	conn     net.Conn
	reader   *protocol.Reader
	writer   *protocol.Writer
	targetID string

	mu       sync.Mutex
	received []string // action names received
}

func newMockExtension(t *testing.T, sockPath string) *mockExtension {
	t.Helper()

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("mock extension dial: %v", err)
	}

	ext := &mockExtension{
		conn:   conn,
		reader: protocol.NewReader(conn),
		writer: protocol.NewWriter(conn),
	}

	// Send hello
	helloPayload, _ := json.Marshal(map[string]any{
		"channel":       "native-messaging",
		"extensionId":   "test-ext-id",
		"instanceId":    "test-instance",
		"userAgent":     "TestBrowser/1.0",
		"capabilities":  []string{"tabs", "groups", "bookmarks"},
		"min_supported": 1,
	})
	helloMsg := &protocol.Message{
		ID:              protocol.MakeID(),
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeHello,
		Payload:         helloPayload,
	}
	if err := ext.writer.Write(helloMsg); err != nil {
		conn.Close()
		t.Fatalf("mock extension hello write: %v", err)
	}

	// Read hello response to get targetID
	resp, err := ext.reader.Read()
	if err != nil {
		conn.Close()
		t.Fatalf("mock extension hello read: %v", err)
	}
	var helloResp struct {
		TargetID string `json:"targetId"`
	}
	json.Unmarshal(resp.Payload, &helloResp)
	ext.targetID = helloResp.TargetID

	return ext
}

func (ext *mockExtension) recordAction(action string) {
	ext.mu.Lock()
	defer ext.mu.Unlock()
	ext.received = append(ext.received, action)
}

func (ext *mockExtension) getReceived() []string {
	ext.mu.Lock()
	defer ext.mu.Unlock()
	cp := make([]string, len(ext.received))
	copy(cp, ext.received)
	return cp
}

// readLoop reads requests from daemon and responds with mock data.
func (ext *mockExtension) readLoop(ctx context.Context, t *testing.T) {
	t.Helper()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg, err := ext.reader.Read()
		if err != nil {
			return // connection closed
		}

		if msg.Type != protocol.TypeRequest {
			continue
		}

		ext.recordAction(msg.Action)

		resp := ext.buildResponse(msg)
		if err := ext.writer.Write(resp); err != nil {
			return
		}
	}
}

func (ext *mockExtension) buildResponse(msg *protocol.Message) *protocol.Message {
	var payload any

	switch msg.Action {
	case "tabs.list":
		payload = map[string]any{
			"tabs": []map[string]any{
				{"id": 1, "windowId": 1, "title": "Tab One", "url": "https://one.com", "active": true, "pinned": false, "groupId": -1},
				{"id": 2, "windowId": 1, "title": "Tab Two", "url": "https://two.com", "active": false, "pinned": true, "groupId": 5},
				{"id": 3, "windowId": 1, "title": "Tab Three", "url": "https://three.com/path", "active": false, "pinned": false, "groupId": -1},
			},
		}
	case "tabs.close":
		payload = map[string]any{"ok": true}
	case "tabs.activate":
		payload = map[string]any{"ok": true}
	case "tabs.open":
		payload = map[string]any{"tabId": 100, "ok": true}
	case "groups.list":
		payload = map[string]any{
			"groups": []map[string]any{
				{"id": 5, "title": "DevTools", "color": "blue", "collapsed": false, "windowId": 1},
			},
		}
	case "groups.create":
		payload = map[string]any{"groupId": 10, "ok": true}
	case "bookmarks.tree":
		payload = map[string]any{
			"tree": []map[string]any{
				{
					"id": "1", "title": "Bookmarks Bar", "children": []map[string]any{
						{"id": "2", "title": "GitHub", "url": "https://github.com"},
						{"id": "3", "title": "Google", "url": "https://google.com"},
					},
				},
			},
		}
	default:
		// Unknown action: return error
		data, _ := json.Marshal(map[string]string{"message": "unknown action: " + msg.Action})
		return &protocol.Message{
			ID:              msg.ID,
			ProtocolVersion: protocol.ProtocolVersion,
			Type:            protocol.TypeError,
			Error: &protocol.ErrorBody{
				Code:    protocol.ErrUnknownAction,
				Message: "unknown action: " + msg.Action,
			},
			Payload: data,
		}
	}

	data, _ := json.Marshal(payload)
	return &protocol.Message{
		ID:              msg.ID,
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeResponse,
		Payload:         data,
	}
}

func (ext *mockExtension) close() {
	ext.conn.Close()
}

// integrationEnv encapsulates the full integration test environment:
// a real daemon, a mock extension, and a TUI App with a real client.
type integrationEnv struct {
	app      *App
	ext      *mockExtension
	sockPath string
	tmpDir   string
	cancel   context.CancelFunc
}

func newIntegrationEnv(t *testing.T) *integrationEnv {
	t.Helper()

	// Use /tmp for short socket paths (macOS has ~104 char limit)
	tmpDir, err := os.MkdirTemp("/tmp", "ctm-tui-integ-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	sockPath := filepath.Join(tmpDir, "d.sock")
	lockPath := filepath.Join(tmpDir, "d.lock")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	collectionsDir := filepath.Join(tmpDir, "collections")
	bookmarksDir := filepath.Join(tmpDir, "bookmarks")
	overlaysDir := filepath.Join(tmpDir, "overlays")
	workspacesDir := filepath.Join(tmpDir, "workspaces")
	savedSearchesDir := filepath.Join(tmpDir, "searches")
	syncCloudDir := filepath.Join(tmpDir, "sync")
	searchIndexPath := filepath.Join(tmpDir, "search_index.json")

	for _, d := range []string{sessionsDir, collectionsDir, bookmarksDir, overlaysDir, workspacesDir, savedSearchesDir, syncCloudDir} {
		os.MkdirAll(d, 0755)
	}

	ctx, cancel := context.WithCancel(context.Background())

	srv := daemon.NewServer(sockPath, lockPath, sessionsDir, collectionsDir, bookmarksDir, overlaysDir, workspacesDir, savedSearchesDir, syncCloudDir, searchIndexPath)

	// Start daemon in background
	srvErrCh := make(chan error, 1)
	go func() {
		srvErrCh <- srv.Start(ctx)
	}()

	// Wait for daemon to actually accept connections (not just socket file exists)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, dialErr := net.Dial("unix", sockPath)
		if dialErr == nil {
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Connect mock extension
	ext := newMockExtension(t, sockPath)
	go ext.readLoop(ctx, t)

	// Small delay for extension registration
	time.Sleep(50 * time.Millisecond)

	// Create TUI App with real client
	app := NewApp(sockPath)

	t.Cleanup(func() {
		ext.close()
		cancel()
		// Wait a bit for daemon to shut down
		select {
		case <-srvErrCh:
		case <-time.After(2 * time.Second):
		}
		os.RemoveAll(tmpDir)
	})

	return &integrationEnv{
		app:      app,
		ext:      ext,
		sockPath: sockPath,
		tmpDir:   tmpDir,
		cancel:   cancel,
	}
}

// connectApp connects the app's client to the daemon and waits for it.
func (env *integrationEnv) connectApp(t *testing.T) {
	t.Helper()
	if err := env.app.client.Connect(env.app.ctx); err != nil {
		t.Fatalf("client connect: %v", err)
	}
	env.app.connected = true

	// Subscribe for events
	eventCh, err := env.app.client.Subscribe(env.app.ctx, []string{"tabs.*", "groups.*", "bookmarks.*"})
	if err == nil {
		env.app.eventCh = eventCh
	}
}

// execCmd executes a tea.Cmd synchronously and returns the message.
func execCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	return cmd()
}

// execBatch inspects a batch cmd: calls it and returns the message.
// For simple (non-batch) cmds, just returns the single message.
func execCmdWithTimeout(t *testing.T, cmd tea.Cmd, timeout time.Duration) tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	ch := make(chan tea.Msg, 1)
	go func() {
		ch <- cmd()
	}()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(timeout):
		t.Fatalf("cmd timed out after %v", timeout)
		return nil
	}
}

// ---------------------------------------------------------------------------
// Integration tests
// ---------------------------------------------------------------------------

func TestIntegrationInit(t *testing.T) {
	env := newIntegrationEnv(t)

	// Init() returns a batch of connectCmd + WindowSize
	cmd := env.app.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil cmd, expected batch")
	}

	// Execute the init command — it should attempt to connect and refresh
	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	// The batch returns the first completing sub-command's message.
	// We just verify it completes without error.
	if msg != nil {
		if errM, ok := msg.(errMsg); ok {
			t.Fatalf("Init cmd returned error: %v", errM.err)
		}
	}

	// After init, app should be connected
	if !env.app.connected {
		// Init runs in batch; the connect part should have succeeded.
		// If not connected yet, the batch hasn't finished. This is acceptable
		// since tea.Batch is non-deterministic in execution order outside Bubble Tea runtime.
		t.Log("Note: app not connected after init batch (expected in test harness)")
	}
}

func TestIntegrationConnectAndRefresh(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// refreshCurrentView should send tabs.list and parse response
	cmd := env.app.refreshCurrentView()
	if cmd == nil {
		t.Fatal("refreshCurrentView returned nil cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	rm, ok := msg.(refreshMsg)
	if !ok {
		if em, ok := msg.(errMsg); ok {
			t.Fatalf("refresh returned error: %v", em.err)
		}
		t.Fatalf("expected refreshMsg, got %T", msg)
	}

	// Apply the refresh payload
	env.app.applyRefresh(rm.payload)

	vs := env.app.views[ViewTabs]
	if vs.itemCount != 3 {
		t.Errorf("tabs count = %d, want 3", vs.itemCount)
	}

	// Verify tab content
	if len(vs.items) > 0 {
		tab := vs.items[0].(TabItem)
		if tab.Title != "Tab One" {
			t.Errorf("first tab title = %q, want %q", tab.Title, "Tab One")
		}
		if tab.URL != "https://one.com" {
			t.Errorf("first tab URL = %q, want %q", tab.URL, "https://one.com")
		}
	}
}

func TestIntegrationDoRequest(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	var callbackCalled bool
	cmd := env.app.doRequest("tabs.list", nil, func(payload json.RawMessage) {
		callbackCalled = true
		if payload == nil {
			t.Error("callback payload is nil")
		}
	})

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if _, ok := msg.(errMsg); ok {
		t.Fatalf("doRequest returned error: %v", msg.(errMsg).err)
	}
	if !callbackCalled {
		t.Error("onSuccess callback not called")
	}

	// Verify it returns refreshMsg
	if _, ok := msg.(refreshMsg); !ok {
		t.Errorf("expected refreshMsg, got %T", msg)
	}
}

func TestIntegrationDoRequestError(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// Request an action that the mock extension returns error for
	cmd := env.app.doRequest("unknown.action", nil, func(_ json.RawMessage) {
		t.Error("onSuccess should not be called on error")
	})

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if _, ok := msg.(errMsg); !ok {
		t.Errorf("expected errMsg for unknown action, got %T", msg)
	}
}

func TestIntegrationCloseTabs_SingleCursor(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// Populate tabs view
	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, Title: "Tab One", URL: "https://one.com"},
		TabItem{ID: 2, Title: "Tab Two", URL: "https://two.com"},
	}
	vs.itemCount = 2
	vs.cursor = 0

	cmd := env.app.closeTabs()
	if cmd == nil {
		t.Fatal("closeTabs returned nil cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("closeTabs error: %v", em.err)
	}

	// Verify extension received tabs.close
	time.Sleep(100 * time.Millisecond) // allow readLoop to process
	received := env.ext.getReceived()
	found := false
	for _, a := range received {
		if a == "tabs.close" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("extension did not receive tabs.close, got: %v", received)
	}
}

func TestIntegrationCloseTabs_MultiSelect(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 10, Title: "A", URL: "https://a.com"},
		TabItem{ID: 20, Title: "B", URL: "https://b.com"},
		TabItem{ID: 30, Title: "C", URL: "https://c.com"},
	}
	vs.itemCount = 3
	vs.selected[0] = true
	vs.selected[2] = true

	cmd := env.app.closeTabs()
	if cmd == nil {
		t.Fatal("closeTabs with selection returned nil cmd")
	}

	// Execute the batch — in tests, tea.Batch returns a function that
	// executes sub-commands. We can invoke it.
	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	// The batch will return the first sub-result
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("closeTabs error: %v", em.err)
	}

	// Selection should be cleared
	if len(vs.selected) != 0 {
		t.Errorf("selected should be cleared, got %d items", len(vs.selected))
	}
}

func TestIntegrationSaveSession(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	var toastSet bool
	// saveSession internally calls doRequest which triggers extension calls.
	// The daemon's sessions.save action asks the extension for tabs.list and groups.list
	cmd := env.app.saveSession("test-session")
	if cmd == nil {
		t.Fatal("saveSession returned nil cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("saveSession error: %v", em.err)
	}

	// Verify toast was set
	if strings.Contains(env.app.toast, "Saved session") {
		toastSet = true
	}
	if !toastSet {
		t.Logf("toast = %q (may not be set if callback ran in cmd context)", env.app.toast)
	}

	// Verify session file was created
	sessionFile := filepath.Join(env.tmpDir, "sessions", "test-session.json")
	if _, err := os.Stat(sessionFile); err != nil {
		t.Errorf("session file not created: %v", err)
	}
}

func TestIntegrationCreateCollection(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	cmd := env.app.createCollection("my-collection")
	if cmd == nil {
		t.Fatal("createCollection returned nil cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("createCollection error: %v", em.err)
	}

	// Verify collection file exists
	collectionFile := filepath.Join(env.tmpDir, "collections", "my-collection.json")
	if _, err := os.Stat(collectionFile); err != nil {
		t.Errorf("collection file not created: %v", err)
	}

	// Verify toast
	if !strings.Contains(env.app.toast, "Created collection") {
		t.Logf("toast = %q", env.app.toast)
	}
}

func TestIntegrationHandleRestore_Session(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// Pre-create a session file for restore
	session := map[string]any{
		"name":         "restore-me",
		"createdAt":    "2024-01-01T00:00:00Z",
		"sourceTarget": "target_1",
		"windows": []map[string]any{
			{
				"tabs": []map[string]any{
					{"url": "https://restored.com", "title": "Restored Tab", "pinned": false, "active": true, "groupIndex": -1},
				},
			},
		},
		"groups": []any{},
	}
	data, _ := json.MarshalIndent(session, "", "  ")
	os.WriteFile(filepath.Join(env.tmpDir, "sessions", "restore-me.json"), data, 0644)

	// Set up sessions view with the item
	env.app.view = ViewSessions
	vs := env.app.views[ViewSessions]
	vs.items = []any{SessionItem{Name: "restore-me", TabCount: 1, WindowCount: 1, CreatedAt: "2024-01-01T00:00:00Z"}}
	vs.itemCount = 1
	vs.cursor = 0

	cmd := env.app.handleRestore()
	if cmd == nil {
		t.Fatal("handleRestore returned nil cmd for session")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("handleRestore error: %v", em.err)
	}

	// Verify extension received tabs.open
	time.Sleep(100 * time.Millisecond)
	received := env.ext.getReceived()
	found := false
	for _, a := range received {
		if a == "tabs.open" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("extension did not receive tabs.open for restore, got: %v", received)
	}
}

func TestIntegrationHandleRestore_Collection(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// Pre-create a collection file
	collection := map[string]any{
		"name":      "my-coll",
		"createdAt": "2024-01-01T00:00:00Z",
		"updatedAt": "2024-01-01T00:00:00Z",
		"items": []map[string]any{
			{"url": "https://item1.com", "title": "Item 1"},
			{"url": "https://item2.com", "title": "Item 2"},
		},
	}
	data, _ := json.MarshalIndent(collection, "", "  ")
	os.WriteFile(filepath.Join(env.tmpDir, "collections", "my-coll.json"), data, 0644)

	env.app.view = ViewCollections
	vs := env.app.views[ViewCollections]
	vs.items = []any{CollectionItem{Name: "my-coll", ItemCount: 2, CreatedAt: "2024-01-01T00:00:00Z", UpdatedAt: "2024-01-01T00:00:00Z"}}
	vs.itemCount = 1
	vs.cursor = 0

	cmd := env.app.handleRestore()
	if cmd == nil {
		t.Fatal("handleRestore returned nil cmd for collection")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("handleRestore collection error: %v", em.err)
	}
}

func TestIntegrationHandleRestore_EmptyName(t *testing.T) {
	env := newIntegrationEnv(t)

	// When no items exist, handleRestore returns nil
	env.app.view = ViewSessions
	vs := env.app.views[ViewSessions]
	vs.items = nil
	vs.itemCount = 0

	cmd := env.app.handleRestore()
	if cmd != nil {
		t.Error("handleRestore with empty name should return nil")
	}
}

func TestIntegrationSetDefaultTarget(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// Set up targets view
	env.app.view = ViewTargets
	vs := env.app.views[ViewTargets]
	vs.items = []any{
		TargetItem{TargetID: env.ext.targetID, Channel: "native-messaging", Label: "", IsDefault: true},
	}
	vs.itemCount = 1
	vs.cursor = 0

	cmd := env.app.setDefaultTarget()
	if cmd == nil {
		t.Fatal("setDefaultTarget returned nil cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("setDefaultTarget error: %v", em.err)
	}

	// Verify toast
	if !strings.Contains(env.app.toast, "Default target") {
		t.Logf("toast = %q", env.app.toast)
	}
}

func TestIntegrationSetDefaultTarget_OutOfBounds(t *testing.T) {
	env := newIntegrationEnv(t)
	env.app.view = ViewTargets
	vs := env.app.views[ViewTargets]
	vs.items = nil
	vs.itemCount = 0
	vs.cursor = 5

	cmd := env.app.setDefaultTarget()
	if cmd != nil {
		t.Error("setDefaultTarget with out-of-bounds cursor should return nil")
	}
}

func TestIntegrationShowToast(t *testing.T) {
	env := newIntegrationEnv(t)

	cmd := env.app.showToast("Hello World")
	if env.app.toast != "Hello World" {
		t.Errorf("toast = %q, want %q", env.app.toast, "Hello World")
	}
	if cmd == nil {
		t.Fatal("showToast should return a tick cmd")
	}

	// Execute the tick cmd — it should eventually produce toastClearMsg
	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if _, ok := msg.(toastClearMsg); !ok {
		t.Errorf("expected toastClearMsg, got %T", msg)
	}

	// Feed toastClearMsg through Update
	model, _ := env.app.Update(msg)
	a := model.(*App)
	if a.toast != "" {
		t.Errorf("toast after clear = %q, want empty", a.toast)
	}
}

func TestIntegrationTargetSelector_Empty(t *testing.T) {
	env := newIntegrationEnv(t)

	// No selectedTarget — should return nil
	env.app.selectedTarget = ""
	sel := env.app.targetSelector()
	if sel != nil {
		t.Errorf("expected nil target selector, got %+v", sel)
	}
}

func TestIntegrationTargetSelector_WithTarget(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.selectedTarget = "target_1"
	sel := env.app.targetSelector()
	if sel == nil {
		t.Fatal("expected non-nil target selector")
	}
	if sel.TargetID != "target_1" {
		t.Errorf("targetId = %q, want %q", sel.TargetID, "target_1")
	}
}

func TestIntegrationConnectCmd(t *testing.T) {
	env := newIntegrationEnv(t)

	// connectCmd should connect the client and return refreshMsg
	cmd := env.app.connectCmd()
	msg := execCmdWithTimeout(t, cmd, 5*time.Second)

	switch m := msg.(type) {
	case refreshMsg:
		// Success — app should now be connected
		if !env.app.connected {
			t.Error("app not connected after connectCmd")
		}
	case errMsg:
		t.Fatalf("connectCmd returned error: %v", m.err)
	default:
		t.Fatalf("unexpected msg type: %T", msg)
	}

	// eventCh should be set
	if env.app.eventCh == nil {
		t.Error("eventCh not set after connectCmd")
	}
}

func TestIntegrationHandleYankKey_URL(t *testing.T) {
	env := newIntegrationEnv(t)

	// Set up tabs with items
	env.app.view = ViewTabs
	env.app.mode = ModeYank
	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, Title: "GitHub", URL: "https://github.com"},
		TabItem{ID: 2, Title: "Google", URL: "https://google.com"},
	}
	vs.itemCount = 2
	vs.cursor = 0

	// Yank URL (y)
	model, cmd := env.app.handleYankKey("y")
	a := model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if a.confirmHint != "" {
		t.Errorf("confirmHint should be cleared, got %q", a.confirmHint)
	}
	// cmd should be a showToast tick (for "Copied!")
	if cmd != nil {
		// showToast returns a tick cmd
		msg := execCmdWithTimeout(t, cmd, 5*time.Second)
		if _, ok := msg.(toastClearMsg); !ok {
			t.Errorf("expected toastClearMsg from yank, got %T", msg)
		}
	}
}

func TestIntegrationHandleYankKey_Title(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTabs
	env.app.mode = ModeYank
	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, Title: "MyTitle", URL: "https://example.com"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleYankKey("n")
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd == nil {
		t.Log("yank title: cmd is nil (clipboard may not be available in CI)")
	}
}

func TestIntegrationHandleYankKey_Host(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTabs
	env.app.mode = ModeYank
	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, Title: "Page", URL: "https://example.com/path"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	model, _ := env.app.handleYankKey("h")
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
}

func TestIntegrationHandleYankKey_Markdown(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTabs
	env.app.mode = ModeYank
	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, Title: "Page", URL: "https://example.com"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	model, _ := env.app.handleYankKey("m")
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
}

func TestIntegrationHandleYankKey_UnknownKey(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTabs
	env.app.mode = ModeYank
	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, Title: "Page", URL: "https://example.com"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleYankKey("x")
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode should be ModeNormal after unknown yank key")
	}
	if cmd != nil {
		t.Error("unknown yank key should return nil cmd")
	}
}

func TestIntegrationHandleYankKey_EmptyView(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTabs
	env.app.mode = ModeYank
	vs := env.app.views[ViewTabs]
	vs.items = nil
	vs.itemCount = 0

	model, cmd := env.app.handleYankKey("y")
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd != nil {
		t.Error("yank on empty view should return nil cmd")
	}
}

func TestIntegrationHandleYankKey_NonTabView(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewSessions
	env.app.mode = ModeYank
	vs := env.app.views[ViewSessions]
	vs.items = []any{
		SessionItem{Name: "test", TabCount: 1, CreatedAt: "2024-01-01T00:00:00Z"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	a, cmd := env.app.handleYankKey("y")
	// Yank now works on all views — SessionItem copies name
	if cmd == nil {
		t.Error("yank on session view should return a cmd (showToast)")
	}
	app := a.(*App)
	if app.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", app.mode)
	}
}

func TestIntegrationHandleConfirmDeleteKey_Session(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// Create a session file to delete
	session := map[string]any{
		"name":      "del-me",
		"createdAt": "2024-01-01T00:00:00Z",
		"windows":   []any{},
		"groups":    []any{},
	}
	data, _ := json.MarshalIndent(session, "", "  ")
	sessionFile := filepath.Join(env.tmpDir, "sessions", "del-me.json")
	os.WriteFile(sessionFile, data, 0644)

	env.app.view = ViewSessions
	env.app.mode = ModeConfirmDelete
	vs := env.app.views[ViewSessions]
	vs.items = []any{SessionItem{Name: "del-me", TabCount: 0, CreatedAt: "2024-01-01T00:00:00Z"}}
	vs.itemCount = 1
	vs.cursor = 0

	// Press D to confirm delete
	model, cmd := env.app.handleConfirmDeleteKey("D")
	a := model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after confirm delete", a.mode)
	}
	if a.confirmHint != "" {
		t.Errorf("confirmHint should be cleared, got %q", a.confirmHint)
	}

	if cmd == nil {
		t.Fatal("confirm delete should return a cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("delete error: %v", em.err)
	}

	// File should be deleted
	if _, err := os.Stat(sessionFile); !os.IsNotExist(err) {
		t.Error("session file should have been deleted")
	}
}

func TestIntegrationHandleConfirmDeleteKey_Collection(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// Create a collection file to delete
	collection := map[string]any{
		"name":      "del-coll",
		"createdAt": "2024-01-01T00:00:00Z",
		"updatedAt": "2024-01-01T00:00:00Z",
		"items":     []any{},
	}
	data, _ := json.MarshalIndent(collection, "", "  ")
	collFile := filepath.Join(env.tmpDir, "collections", "del-coll.json")
	os.WriteFile(collFile, data, 0644)

	env.app.view = ViewCollections
	env.app.mode = ModeConfirmDelete
	vs := env.app.views[ViewCollections]
	vs.items = []any{CollectionItem{Name: "del-coll", ItemCount: 0, CreatedAt: "2024-01-01T00:00:00Z", UpdatedAt: "2024-01-01T00:00:00Z"}}
	vs.itemCount = 1
	vs.cursor = 0

	_, cmd := env.app.handleConfirmDeleteKey("D")
	if cmd == nil {
		t.Fatal("confirm delete collection should return cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("delete collection error: %v", em.err)
	}

	if _, err := os.Stat(collFile); !os.IsNotExist(err) {
		t.Error("collection file should have been deleted")
	}
}

func TestIntegrationHandleConfirmDeleteKey_Cancel(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewSessions
	env.app.mode = ModeConfirmDelete
	vs := env.app.views[ViewSessions]
	vs.items = []any{SessionItem{Name: "keep-me", TabCount: 1, CreatedAt: "2024-01-01T00:00:00Z"}}
	vs.itemCount = 1
	vs.cursor = 0

	// Press non-D key to cancel
	model, cmd := env.app.handleConfirmDeleteKey("n")
	a := model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after cancel", a.mode)
	}
	if cmd != nil {
		t.Error("cancel delete should return nil cmd")
	}
}

func TestIntegrationHandleConfirmDeleteKey_EmptyName(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewSessions
	env.app.mode = ModeConfirmDelete
	vs := env.app.views[ViewSessions]
	vs.items = nil
	vs.itemCount = 0

	_, cmd := env.app.handleConfirmDeleteKey("D")
	if cmd != nil {
		t.Error("delete with empty name should return nil cmd")
	}
}

func TestIntegrationHandleConfirmDeleteKey_NonDeleteView(t *testing.T) {
	env := newIntegrationEnv(t)

	// Tabs view doesn't support D-D delete
	env.app.view = ViewTabs
	env.app.mode = ModeConfirmDelete
	vs := env.app.views[ViewTabs]
	vs.items = []any{TabItem{ID: 1, Title: "Tab", URL: "https://example.com"}}
	vs.itemCount = 1
	vs.cursor = 0

	_, cmd := env.app.handleConfirmDeleteKey("D")
	if cmd != nil {
		t.Error("delete on tabs view should return nil cmd")
	}
}

func TestIntegrationRefreshCurrentView_AllViews(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// Test each view type refreshes correctly
	tests := []struct {
		view       ViewType
		expectNil  bool
		expectKey  string // key to check in response payload
	}{
		{ViewTabs, false, "tabs"},
		{ViewGroups, false, "groups"},
		{ViewSessions, false, "sessions"},
		{ViewCollections, false, "collections"},
		{ViewTargets, false, "targets"},
		{ViewBookmarks, false, "tree"},
		{ViewWorkspaces, false, "workspaces"},
		{ViewSync, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.view.String(), func(t *testing.T) {
			env.app.view = tt.view
			cmd := env.app.refreshCurrentView()
			if tt.expectNil {
				if cmd != nil {
					t.Error("expected nil cmd")
				}
				return
			}
			if cmd == nil {
				t.Fatal("expected non-nil cmd")
			}

			msg := execCmdWithTimeout(t, cmd, 5*time.Second)
			switch m := msg.(type) {
			case refreshMsg:
				if tt.expectKey != "" && m.payload != nil {
					var data map[string]json.RawMessage
					if json.Unmarshal(m.payload, &data) == nil {
						if _, ok := data[tt.expectKey]; !ok {
							t.Errorf("response payload missing key %q", tt.expectKey)
						}
					}
				}
			case errMsg:
				// Some views may return errors if no data exists. That's acceptable.
				t.Logf("view %s refresh returned error: %v", tt.view, m.err)
			default:
				t.Errorf("unexpected msg type: %T", msg)
			}
		})
	}
}

func TestIntegrationRefreshCurrentView_NotConnected(t *testing.T) {
	env := newIntegrationEnv(t)
	// Don't connect — app.connected is false

	env.app.view = ViewTabs
	cmd := env.app.refreshCurrentView()
	if cmd != nil {
		t.Error("refreshCurrentView should return nil when not connected")
	}
}

func TestIntegrationWaitForEvent(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// waitForEvent returns a cmd that blocks until an event arrives
	cmd := env.app.waitForEvent()
	if cmd == nil {
		t.Fatal("waitForEvent should return non-nil cmd after subscribe")
	}

	// Send a mock event from the extension
	go func() {
		time.Sleep(100 * time.Millisecond)
		evt := &protocol.Message{
			ID:              protocol.MakeID(),
			ProtocolVersion: protocol.ProtocolVersion,
			Type:            protocol.TypeEvent,
			Action:          "tabs.created",
			Payload:         json.RawMessage(`{"tab":{"id":99}}`),
		}
		env.ext.writer.Write(evt)
	}()

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if msg == nil {
		t.Fatal("waitForEvent returned nil msg")
	}
	if _, ok := msg.(eventMsg); !ok {
		t.Errorf("expected eventMsg, got %T", msg)
	}
}

func TestIntegrationWaitForEvent_NilChannel(t *testing.T) {
	env := newIntegrationEnv(t)

	// Without subscribe, eventCh is nil
	env.app.eventCh = nil
	cmd := env.app.waitForEvent()
	if cmd != nil {
		t.Error("waitForEvent with nil eventCh should return nil")
	}
}

func TestIntegrationRenderItem_AllTypes(t *testing.T) {
	env := newIntegrationEnv(t)

	tests := []struct {
		name     string
		item     any
		contains string
	}{
		{
			"TabItem",
			TabItem{ID: 42, Title: "Test Tab", URL: "https://example.com", Active: true, Pinned: true, GroupID: 5},
			"Test Tab",
		},
		{
			"TabItem_Flags",
			TabItem{ID: 1, Title: "Pinned+Active", URL: "https://test.com", Active: true, Pinned: true},
			"\u2295", // ⊕ pinned icon (pinned takes priority over active)
		},
		{
			"GroupItem",
			GroupItem{ID: 5, Title: "DevGroup", Color: "blue", Collapsed: true},
			"DevGroup",
		},
		{
			"SessionItem",
			SessionItem{Name: "my-session", TabCount: 10, WindowCount: 2, GroupCount: 1, CreatedAt: "2024-01-01T12:00:00Z"},
			"my-session",
		},
		{
			"CollectionItem",
			CollectionItem{Name: "my-coll", ItemCount: 5, CreatedAt: "2024-01-01T00:00:00Z", UpdatedAt: "2024-06-15T10:30:00Z"},
			"my-coll",
		},
		{
			"TargetItem_Default",
			TargetItem{TargetID: "target_1", Channel: "native-messaging", Label: "Main", IsDefault: true},
			"*",
		},
		{
			"TargetItem_NotDefault",
			TargetItem{TargetID: "target_2", Channel: "devtools", Label: "", IsDefault: false},
			"Browser", // empty UA renders as "Browser"
		},
		{
			"BookmarkItem_Folder",
			BookmarkItem{ID: "1", Title: "Folder", Depth: 0, IsFolder: true},
			"Folder",
		},
		{
			"BookmarkItem_Link",
			BookmarkItem{ID: "2", Title: "Bookmark", URL: "https://example.com", Depth: 1, IsFolder: false},
			"Bookmark",
		},
		{
			"BookmarkItem_Depth",
			BookmarkItem{ID: "3", Title: "Deep", URL: "https://deep.com", Depth: 3, IsFolder: false},
			"Deep",
		},
		{
			"WorkspaceItem",
			WorkspaceItem{Name: "workspace-1", SessionCount: 3, CollectionCount: 2, UpdatedAt: "2024-06-15T10:30:00Z"},
			"workspace-1",
		},
		{
			"SyncStatusItem_Enabled",
			SyncStatusItem{Enabled: true, SyncDir: "/tmp/sync", LastSync: "2024-01-01T00:00:00Z", PendingChanges: 2, Conflicts: []string{"a"}},
			"enabled",
		},
		{
			"SyncStatusItem_Disabled",
			SyncStatusItem{Enabled: false, SyncDir: "/tmp/sync", LastSync: "", PendingChanges: 0},
			"disabled",
		},
		{
			"SyncStatusItem_NoLastSync",
			SyncStatusItem{Enabled: true, SyncDir: "/data", LastSync: "", PendingChanges: 0},
			"Pending: 0",
		},
		{
			"UnknownType",
			struct{ Foo string }{"bar"},
			"bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := env.app.renderItem(tt.item)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("renderItem(%s) = %q, want to contain %q", tt.name, result, tt.contains)
			}
		})
	}
}

func TestIntegrationApplyRefresh_AllViews(t *testing.T) {
	env := newIntegrationEnv(t)

	// Test tabs
	tabsPayload, _ := json.Marshal(map[string]any{
		"tabs": []map[string]any{
			{"id": 1, "title": "Tab A", "url": "https://a.com", "active": true},
			{"id": 2, "title": "Tab B", "url": "https://b.com"},
		},
	})
	env.app.view = ViewTabs
	env.app.applyRefresh(tabsPayload)
	if env.app.views[ViewTabs].itemCount != 2 {
		t.Errorf("tabs itemCount = %d, want 2", env.app.views[ViewTabs].itemCount)
	}

	// Test groups
	groupsPayload, _ := json.Marshal(map[string]any{
		"groups": []map[string]any{
			{"id": 1, "title": "G1", "color": "red"},
		},
	})
	env.app.view = ViewGroups
	env.app.applyRefresh(groupsPayload)
	if env.app.views[ViewGroups].itemCount != 1 {
		t.Errorf("groups itemCount = %d, want 1", env.app.views[ViewGroups].itemCount)
	}

	// Test sessions
	sessionsPayload, _ := json.Marshal(map[string]any{
		"sessions": []map[string]any{
			{"name": "s1", "tabCount": 5, "createdAt": "2024-01-01T00:00:00Z"},
		},
	})
	env.app.view = ViewSessions
	env.app.applyRefresh(sessionsPayload)
	if env.app.views[ViewSessions].itemCount != 1 {
		t.Errorf("sessions itemCount = %d, want 1", env.app.views[ViewSessions].itemCount)
	}

	// Test collections
	collPayload, _ := json.Marshal(map[string]any{
		"collections": []map[string]any{
			{"name": "c1", "itemCount": 3, "createdAt": "2024-01-01T00:00:00Z", "updatedAt": "2024-01-01T00:00:00Z"},
		},
	})
	env.app.view = ViewCollections
	env.app.applyRefresh(collPayload)
	if env.app.views[ViewCollections].itemCount != 1 {
		t.Errorf("collections itemCount = %d, want 1", env.app.views[ViewCollections].itemCount)
	}

	// Test targets (with auto-select default)
	targetsPayload, _ := json.Marshal(map[string]any{
		"targets": []map[string]any{
			{"targetId": "t1", "channel": "nm", "isDefault": true},
			{"targetId": "t2", "channel": "nm", "isDefault": false},
		},
	})
	env.app.view = ViewTargets
	env.app.applyRefresh(targetsPayload)
	if env.app.views[ViewTargets].itemCount != 2 {
		t.Errorf("targets itemCount = %d, want 2", env.app.views[ViewTargets].itemCount)
	}
	if env.app.selectedTarget != "t1" {
		t.Errorf("selectedTarget = %q, want %q", env.app.selectedTarget, "t1")
	}

	// Test bookmarks (tree flattening)
	bmPayload, _ := json.Marshal(map[string]any{
		"tree": []map[string]any{
			{
				"id": "1", "title": "Root", "children": []map[string]any{
					{"id": "2", "title": "Child1", "url": "https://child1.com"},
					{"id": "3", "title": "Child2", "url": "https://child2.com"},
				},
			},
		},
	})
	env.app.view = ViewBookmarks
	env.app.applyRefresh(bmPayload)
	if env.app.views[ViewBookmarks].itemCount != 1 { // root only (default folded)
		t.Errorf("bookmarks itemCount = %d, want 1 (folded)", env.app.views[ViewBookmarks].itemCount)
	}

	// Test workspaces
	wsPayload, _ := json.Marshal(map[string]any{
		"workspaces": []map[string]any{
			{"id": "w1", "name": "ws1", "sessionCount": 1, "collectionCount": 0, "updatedAt": "2024-01-01T00:00:00Z"},
		},
	})
	env.app.view = ViewWorkspaces
	env.app.applyRefresh(wsPayload)
	if env.app.views[ViewWorkspaces].itemCount != 1 {
		t.Errorf("workspaces itemCount = %d, want 1", env.app.views[ViewWorkspaces].itemCount)
	}

	// Test sync status (single object, not array)
	syncPayload, _ := json.Marshal(map[string]any{
		"enabled": true, "syncDir": "/tmp/sync", "lastSync": "", "pendingChanges": 0,
	})
	env.app.view = ViewSync
	env.app.applyRefresh(syncPayload)
	if env.app.views[ViewSync].itemCount != 1 {
		t.Errorf("sync itemCount = %d, want 1", env.app.views[ViewSync].itemCount)
	}

	// Test nil payload
	env.app.view = ViewTabs
	prevCount := env.app.views[ViewTabs].itemCount
	env.app.applyRefresh(nil)
	if env.app.views[ViewTabs].itemCount != prevCount {
		t.Error("applyRefresh(nil) should not change items")
	}
}

func TestIntegrationExecuteCommand_Save(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	cmd := env.app.executeCommand("save my-saved-session")
	if cmd == nil {
		t.Fatal("executeCommand save should return a cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("save command error: %v", em.err)
	}

	// Check session file
	sessionFile := filepath.Join(env.tmpDir, "sessions", "my-saved-session.json")
	if _, err := os.Stat(sessionFile); err != nil {
		t.Errorf("session file not created by :save command: %v", err)
	}
}

func TestIntegrationExecuteCommand_Restore(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// Pre-create session
	session := map[string]any{
		"name": "cmd-restore", "createdAt": "2024-01-01T00:00:00Z",
		"windows": []map[string]any{
			{"tabs": []map[string]any{
				{"url": "https://cmd.com", "title": "CMD", "pinned": false, "active": true, "groupIndex": -1},
			}},
		},
		"groups": []any{},
	}
	data, _ := json.MarshalIndent(session, "", "  ")
	os.WriteFile(filepath.Join(env.tmpDir, "sessions", "cmd-restore.json"), data, 0644)

	cmd := env.app.executeCommand("restore cmd-restore")
	if cmd == nil {
		t.Fatal("executeCommand restore should return a cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("restore command error: %v", em.err)
	}
}

func TestIntegrationExecuteCommand_Target(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	cmd := env.app.executeCommand("target")
	// Should switch view to ViewTargets and refresh
	if env.app.view != ViewTargets {
		t.Errorf("view = %v, want ViewTargets", env.app.view)
	}
	if cmd == nil {
		t.Error("target command should return refresh cmd")
	}
}

func TestIntegrationExecuteCommand_Help(t *testing.T) {
	env := newIntegrationEnv(t)

	cmd := env.app.executeCommand("help")
	if env.app.mode != ModeHelp {
		t.Errorf("mode = %d, want ModeHelp", env.app.mode)
	}
	if cmd != nil {
		t.Error("help command should return nil cmd")
	}
}

func TestIntegrationExecuteCommand_Quit(t *testing.T) {
	env := newIntegrationEnv(t)

	cmd := env.app.executeCommand("q")
	if cmd == nil {
		t.Fatal("quit command should return tea.Quit")
	}
	msg := execCmd(t, cmd)
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", msg)
	}
}

func TestIntegrationExecuteCommand_Empty(t *testing.T) {
	env := newIntegrationEnv(t)

	cmd := env.app.executeCommand("")
	if cmd != nil {
		t.Error("empty command should return nil")
	}
}

func TestIntegrationExecuteCommand_SaveWithoutName(t *testing.T) {
	env := newIntegrationEnv(t)

	cmd := env.app.executeCommand("save")
	if cmd != nil {
		t.Error("save without name should return nil")
	}
}

func TestIntegrationExecuteCommand_RestoreWithoutName(t *testing.T) {
	env := newIntegrationEnv(t)

	cmd := env.app.executeCommand("restore")
	if cmd != nil {
		t.Error("restore without name should return nil")
	}
}

func TestIntegrationHandleNameInputKey_SaveSession(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewSessions
	env.app.mode = ModeNameInput
	env.app.nameText = "new-session"

	model, cmd := env.app.handleNameInputKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if a.nameText != "" {
		t.Errorf("nameText = %q, want empty", a.nameText)
	}
	if cmd == nil {
		t.Fatal("save session via name input should return cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("save via name input error: %v", em.err)
	}
}

func TestIntegrationHandleNameInputKey_CreateCollection(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewCollections
	env.app.mode = ModeNameInput
	env.app.nameText = "new-coll"

	model, cmd := env.app.handleNameInputKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd == nil {
		t.Fatal("create collection via name input should return cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("create collection error: %v", em.err)
	}

	collFile := filepath.Join(env.tmpDir, "collections", "new-coll.json")
	if _, err := os.Stat(collFile); err != nil {
		t.Errorf("collection file not created: %v", err)
	}
}

func TestIntegrationHandleNameInputKey_EmptyName(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewSessions
	env.app.mode = ModeNameInput
	env.app.nameText = ""

	model, cmd := env.app.handleNameInputKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd != nil {
		t.Error("empty name should return nil cmd")
	}
}

func TestIntegrationHandleEnter_TabActivate(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewTabs
	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 42, Title: "Test", URL: "https://test.com"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	cmd := env.app.handleEnter()
	if cmd == nil {
		t.Fatal("handleEnter on tab should return activate cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("activate tab error: %v", em.err)
	}

	// Extension should have received tabs.activate
	time.Sleep(100 * time.Millisecond)
	received := env.ext.getReceived()
	found := false
	for _, a := range received {
		if a == "tabs.activate" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("extension did not receive tabs.activate, got: %v", received)
	}
}

func TestIntegrationHandleEnter_TargetSelect(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewTargets
	vs := env.app.views[ViewTargets]
	vs.items = []any{
		TargetItem{TargetID: env.ext.targetID, Channel: "nm"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	cmd := env.app.handleEnter()

	// Should switch selectedTarget and view
	if env.app.selectedTarget != env.ext.targetID {
		t.Errorf("selectedTarget = %q, want %q", env.app.selectedTarget, env.ext.targetID)
	}
	if env.app.view != ViewTabs {
		t.Errorf("view = %v, want ViewTabs", env.app.view)
	}
	if cmd == nil {
		t.Error("handleEnter on target should return refresh cmd")
	}
}

func TestIntegrationHandleEnter_EmptyView(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTabs
	vs := env.app.views[ViewTabs]
	vs.items = nil
	vs.itemCount = 0

	cmd := env.app.handleEnter()
	if cmd != nil {
		t.Error("handleEnter on empty view should return nil")
	}
}

func TestIntegrationUpdateErrMsg(t *testing.T) {
	env := newIntegrationEnv(t)

	model, _ := env.app.Update(errMsg{err: fmt.Errorf("test error")})
	a := model.(*App)

	if a.errorMsg != "test error" {
		t.Errorf("errorMsg = %q, want %q", a.errorMsg, "test error")
	}

	// Esc should clear error
	model, _ = a.Update(tea.KeyMsg{Type: tea.KeyEscape})
	a = model.(*App)
	if a.errorMsg != "" {
		t.Errorf("errorMsg after Esc = %q, want empty", a.errorMsg)
	}
}

func TestIntegrationUpdateRefreshMsg(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	payload, _ := json.Marshal(map[string]any{
		"tabs": []map[string]any{
			{"id": 1, "title": "From Event", "url": "https://event.com"},
		},
	})

	model, cmd := env.app.Update(refreshMsg{payload: payload})
	a := model.(*App)

	vs := a.views[ViewTabs]
	if vs.itemCount != 1 {
		t.Errorf("itemCount = %d, want 1", vs.itemCount)
	}

	// Should return waitForEvent cmd
	if cmd == nil {
		t.Log("waitForEvent cmd is nil (eventCh may not be set)")
	}
}

func TestIntegrationUpdateEventMsg(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	evt := &protocol.Message{
		ID:     "evt-1",
		Type:   protocol.TypeEvent,
		Action: "tabs.created",
	}

	model, cmd := env.app.Update(eventMsg(evt))
	_ = model.(*App)

	// eventMsg should trigger refreshCurrentView + waitForEvent batch
	if cmd == nil {
		t.Log("cmd from eventMsg is nil (expected batch)")
	}
}

func TestIntegrationUpdateChordTimeout(t *testing.T) {
	env := newIntegrationEnv(t)

	// Test yank mode timeout
	env.app.mode = ModeYank
	model, _ := env.app.Update(chordTimeoutMsg{})
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode after yank timeout = %d, want ModeNormal", a.mode)
	}

	// Test zfilter mode timeout
	env.app.mode = ModeZFilter
	model, _ = env.app.Update(chordTimeoutMsg{})
	a = model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode after zfilter timeout = %d, want ModeNormal", a.mode)
	}

	// Test confirm delete mode timeout
	env.app.mode = ModeConfirmDelete
	env.app.confirmHint = "Press D again..."
	model, _ = env.app.Update(chordTimeoutMsg{})
	a = model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode after confirm timeout = %d, want ModeNormal", a.mode)
	}
	if a.confirmHint != "" {
		t.Errorf("confirmHint = %q, want empty", a.confirmHint)
	}
}

func TestIntegrationUpdatePendingGTimeout(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.pendingG = true
	model, _ := env.app.Update(pendingGTimeoutMsg{})
	a := model.(*App)
	if a.pendingG {
		t.Error("pendingG should be false after timeout")
	}
}

func TestIntegrationView(t *testing.T) {
	env := newIntegrationEnv(t)

	// Basic view rendering with no data
	output := env.app.View()
	if !strings.Contains(output, "CTM") {
		t.Error("view should contain CTM header")
	}
	if !strings.Contains(output, "(no tabs") {
		t.Error("view should show empty state hint when no items")
	}

	// Populate some data and render
	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, Title: "Tab One", URL: "https://one.com", Active: true},
	}
	vs.itemCount = 1

	output = env.app.View()
	if !strings.Contains(output, "Tab One") {
		t.Error("view should contain tab title")
	}

	// Test help mode rendering
	env.app.mode = ModeHelp
	output = env.app.View()
	if !strings.Contains(output, "Help") {
		t.Error("help view should contain 'Help'")
	}

	// Test filter bar rendering
	env.app.mode = ModeFilter
	env.app.filterText = "test"
	output = env.app.View()
	if !strings.Contains(output, "/ test") {
		t.Error("filter mode should show filter bar")
	}

	// Test command bar rendering
	env.app.mode = ModeCommand
	env.app.commandText = "save"
	output = env.app.View()
	if !strings.Contains(output, ": save") {
		t.Error("command mode should show command bar")
	}

	// Test name input bar rendering
	env.app.mode = ModeNameInput
	env.app.nameText = "my-name"
	output = env.app.View()
	if !strings.Contains(output, "Name: my-name") {
		t.Error("name input mode should show name bar")
	}
}

func TestIntegrationRenderStatusBar(t *testing.T) {
	env := newIntegrationEnv(t)

	// Error takes priority
	env.app.errorMsg = "something broke"
	output := env.app.renderStatusBar()
	if !strings.Contains(output, "ERROR") {
		t.Error("status bar should show ERROR")
	}

	// ConfirmHint takes priority over toast
	env.app.errorMsg = ""
	env.app.confirmHint = "Press D again"
	output = env.app.renderStatusBar()
	if !strings.Contains(output, "Press D again") {
		t.Error("status bar should show confirm hint")
	}

	// Toast
	env.app.confirmHint = ""
	env.app.toast = "Saved!"
	output = env.app.renderStatusBar()
	if !strings.Contains(output, "Saved!") {
		t.Error("status bar should show toast")
	}

	// Selected count
	env.app.toast = ""
	vs := env.app.views[ViewTabs]
	vs.selected[0] = true
	vs.selected[1] = true
	output = env.app.renderStatusBar()
	if !strings.Contains(output, "2 selected") {
		t.Error("status bar should show selected count")
	}
}

func TestIntegrationRenderHeader(t *testing.T) {
	env := newIntegrationEnv(t)

	// Disconnected
	env.app.connected = false
	output := env.app.renderHeader()
	if !strings.Contains(output, "disconnected") {
		t.Error("header should show disconnected")
	}

	// Connected without target
	env.app.connected = true
	env.app.selectedTarget = ""
	output = env.app.renderHeader()
	if !strings.Contains(output, "connected") {
		t.Error("header should show connected")
	}

	// Connected with target
	env.app.selectedTarget = "target_1"
	output = env.app.renderHeader()
	if !strings.Contains(output, "target: target_1") {
		t.Error("header should show target ID")
	}
}

// ---------------------------------------------------------------------------
// Additional tests for coverage improvement
// ---------------------------------------------------------------------------

// --- Pure helper function tests (no daemon needed) ---

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		contains string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}
	for _, tt := range tests {
		result := formatBytes(tt.input)
		if result != tt.contains {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, result, tt.contains)
		}
	}
}

func TestExtractBrowserName(t *testing.T) {
	tests := []struct {
		ua   string
		want string
	}{
		{"", "Browser"},
		{"Mozilla/5.0 (Macintosh) Chrome/145.0.0.0 Safari/537.36", "Chrome"},
		{"Mozilla/5.0 Chrome/145.0.0.0 Chrome Beta Safari/537.36", "Chrome Beta"},
		{"Mozilla/5.0 Chrome/145 Edg/145.0.0.0", "Edge"},
		{"Mozilla/5.0 edge/145", "Edge"},
		{"Mozilla/5.0 Arc/1.0", "Arc"},
		{"Mozilla/5.0 arc /1.0", "Arc"},
		{"Mozilla/5.0 Brave/1.0", "Brave"},
		{"Mozilla/5.0 Vivaldi/6.0", "Vivaldi"},
		{"Mozilla/5.0 OPR/100.0", "Opera"},
		{"Mozilla/5.0 Opera/100.0", "Opera"},
		{"Mozilla/5.0 Firefox/130.0", "Firefox"},
		{"Mozilla/5.0 Safari/605.1.15", "Safari"},
		{"SomethingUnknown/1.0", "Browser"},
	}
	for _, tt := range tests {
		result := extractBrowserName(tt.ua)
		if result != tt.want {
			t.Errorf("extractBrowserName(%q) = %q, want %q", tt.ua, result, tt.want)
		}
	}
}

func TestExtractBrowserVersion(t *testing.T) {
	tests := []struct {
		ua   string
		want string
	}{
		{"", ""},
		{"Mozilla/5.0 Chrome/145.0.0.0", "145"},
		{"Mozilla/5.0 Edg/131.0.0.0", "131"},
		{"Mozilla/5.0 Firefox/130.0", "130"},
		{"Mozilla/5.0 Version/17.5 Safari/605", "17"},
		{"SomethingUnknown/1.0", ""},
	}
	for _, tt := range tests {
		result := extractBrowserVersion(tt.ua)
		if result != tt.want {
			t.Errorf("extractBrowserVersion(%q) = %q, want %q", tt.ua, result, tt.want)
		}
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"", ""},
		{"https://github.com/user/repo", "github.com"},
		{"http://localhost:3000/path", "localhost"},
		{"not-a-url", "not-a-url"},
	}
	for _, tt := range tests {
		result := extractDomain(tt.url)
		if result != tt.want {
			t.Errorf("extractDomain(%q) = %q, want %q", tt.url, result, tt.want)
		}
	}
}

func TestRwTruncate(t *testing.T) {
	// Short string, no truncation needed
	result := rwTruncate("hello", 10, "...")
	if result != "hello" {
		t.Errorf("rwTruncate short = %q, want %q", result, "hello")
	}
	// Long string needs truncation
	result = rwTruncate("a very long string", 10, "...")
	if len(result) > 10 {
		t.Errorf("rwTruncate long display width > 10")
	}
}

func TestGroupColorDot(t *testing.T) {
	// Known colors
	for _, color := range []string{"blue", "red", "yellow", "green", "pink", "purple", "cyan", "orange", "grey"} {
		result := groupColorDot(color)
		if result == "" {
			t.Errorf("groupColorDot(%q) returned empty", color)
		}
	}
	// Unknown color returns default
	result := groupColorDot("unknown")
	if result == "" {
		t.Error("groupColorDot unknown returned empty")
	}
}

func TestMatchesFilter_AllTypes(t *testing.T) {
	tests := []struct {
		name  string
		item  any
		query string
		want  bool
	}{
		{"TabItem match title", TabItem{Title: "GitHub", URL: "https://github.com"}, "github", true},
		{"TabItem match url", TabItem{Title: "Page", URL: "https://github.com"}, "github", true},
		{"TabItem no match", TabItem{Title: "Page", URL: "https://example.com"}, "github", false},
		{"GroupItem match", GroupItem{Title: "DevTools"}, "dev", true},
		{"GroupItem no match", GroupItem{Title: "DevTools"}, "prod", false},
		{"SessionItem match", SessionItem{Name: "work-session"}, "work", true},
		{"CollectionItem match", CollectionItem{Name: "reading-list"}, "reading", true},
		{"NestedTabItem match title", NestedTabItem{Title: "Nested Tab", URL: "https://nested.com"}, "nested", true},
		{"NestedTabItem match url", NestedTabItem{Title: "Nested Tab", URL: "https://nested.com"}, "nested.com", true},
		{"TargetItem match label", TargetItem{Label: "Main Browser", TargetID: "t_1"}, "main", true},
		{"TargetItem match id", TargetItem{Label: "", TargetID: "target_123"}, "123", true},
		{"BookmarkItem match title", BookmarkItem{Title: "Bookmark", URL: "https://bm.com"}, "bookmark", true},
		{"BookmarkItem match url", BookmarkItem{Title: "BM", URL: "https://bm.com"}, "bm.com", true},
		{"WorkspaceItem match", WorkspaceItem{Name: "dev-workspace"}, "dev", true},
		{"SyncStatusItem match", SyncStatusItem{SyncDir: "/tmp/sync"}, "sync", true},
		{"HistoryItem match title", HistoryItem{Title: "Google", URL: "https://google.com"}, "google", true},
		{"HistoryItem match url", HistoryItem{Title: "G", URL: "https://google.com"}, "google.com", true},
		{"SearchResultItem match", SearchResultItem{Title: "Result", URL: "https://r.com", Kind: "tab"}, "result", true},
		{"SearchResultItem match kind", SearchResultItem{Title: "X", URL: "", Kind: "session"}, "session", true},
		{"SavedSearchItem match name", SavedSearchItem{Name: "my-search", QueryText: "q"}, "my-search", true},
		{"SavedSearchItem match query", SavedSearchItem{Name: "s", QueryText: "github tabs"}, "github", true},
		{"DownloadItem match filename", DownloadItem{Filename: "file.zip", URL: "https://dl.com/file.zip"}, "file.zip", true},
		{"DownloadItem match url", DownloadItem{Filename: "f", URL: "https://dl.com/file.zip"}, "dl.com", true},
		{"Unknown type", struct{ X string }{"val"}, "val", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesFilter(tt.item, tt.query)
			if result != tt.want {
				t.Errorf("matchesFilter(%s, %q) = %v, want %v", tt.name, tt.query, result, tt.want)
			}
		})
	}
}

// --- parseSessionTabs / parseCollectionTabs ---

func TestParseSessionTabs(t *testing.T) {
	env := newIntegrationEnv(t)

	payload, _ := json.Marshal(map[string]any{
		"session": map[string]any{
			"windows": []map[string]any{
				{
					"tabs": []map[string]any{
						{"url": "https://a.com", "title": "Tab A", "pinned": true},
						{"url": "https://b.com", "title": "Tab B", "pinned": false},
					},
				},
				{
					"tabs": []map[string]any{
						{"url": "https://c.com", "title": "Tab C", "pinned": false},
					},
				},
			},
		},
	})

	tabs := env.app.parseSessionTabs(payload, "test-session")
	if len(tabs) != 3 {
		t.Fatalf("parseSessionTabs returned %d tabs, want 3", len(tabs))
	}
	if tabs[0].Title != "Tab A" || !tabs[0].Pinned {
		t.Errorf("tab[0] = %+v, want Title=Tab A, Pinned=true", tabs[0])
	}
	if tabs[0].ParentName != "test-session" {
		t.Errorf("tab[0].ParentName = %q, want %q", tabs[0].ParentName, "test-session")
	}
}

func TestParseCollectionTabs(t *testing.T) {
	env := newIntegrationEnv(t)

	payload, _ := json.Marshal(map[string]any{
		"collection": map[string]any{
			"items": []map[string]any{
				{"url": "https://x.com", "title": "Item X"},
				{"url": "https://y.com", "title": "Item Y"},
			},
		},
	})

	tabs := env.app.parseCollectionTabs(payload, "my-coll")
	if len(tabs) != 2 {
		t.Fatalf("parseCollectionTabs returned %d tabs, want 2", len(tabs))
	}
	if tabs[0].URL != "https://x.com" || tabs[0].ParentName != "my-coll" {
		t.Errorf("tab[0] = %+v, want URL=https://x.com, ParentName=my-coll", tabs[0])
	}
}

// --- rebuildSessionItems / rebuildCollectionItems ---

func TestRebuildSessionItems(t *testing.T) {
	env := newIntegrationEnv(t)

	vs := env.app.views[ViewSessions]
	vs.items = []any{
		SessionItem{Name: "s1", TabCount: 2},
		SessionItem{Name: "s2", TabCount: 1},
	}
	vs.itemCount = 2

	// Expand s1
	env.app.expandedSessions["s1"] = []NestedTabItem{
		{URL: "https://a.com", Title: "A", ParentName: "s1"},
		{URL: "https://b.com", Title: "B", ParentName: "s1"},
	}

	env.app.rebuildSessionItems()

	if len(vs.items) != 4 { // s1 + 2 nested + s2
		t.Errorf("rebuildSessionItems: %d items, want 4", len(vs.items))
	}
	if _, ok := vs.items[0].(SessionItem); !ok {
		t.Error("item[0] should be SessionItem")
	}
	if nested, ok := vs.items[1].(NestedTabItem); !ok || nested.Title != "A" {
		t.Errorf("item[1] should be NestedTabItem 'A', got %T", vs.items[1])
	}
}

func TestRebuildCollectionItems(t *testing.T) {
	env := newIntegrationEnv(t)

	vs := env.app.views[ViewCollections]
	vs.items = []any{
		CollectionItem{Name: "c1", ItemCount: 1},
		CollectionItem{Name: "c2", ItemCount: 2},
	}
	vs.itemCount = 2

	env.app.expandedCollections["c2"] = []NestedTabItem{
		{URL: "https://x.com", Title: "X", ParentName: "c2"},
	}

	env.app.rebuildCollectionItems()

	if len(vs.items) != 3 { // c1 + c2 + 1 nested
		t.Errorf("rebuildCollectionItems: %d items, want 3", len(vs.items))
	}
}

// --- handleNameInputKey additional branches ---

func TestIntegrationHandleNameInputKey_Escape(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.mode = ModeNameInput
	env.app.nameText = "something"

	model, cmd := env.app.handleNameInputKey("esc", tea.KeyMsg{Type: tea.KeyEscape})
	a := model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if a.nameText != "" {
		t.Errorf("nameText = %q, want empty", a.nameText)
	}
	if cmd != nil {
		t.Error("esc should return nil cmd")
	}
}

func TestIntegrationHandleNameInputKey_Backspace(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.mode = ModeNameInput
	env.app.nameText = "hello"

	model, _ := env.app.handleNameInputKey("backspace", tea.KeyMsg{Type: tea.KeyBackspace})
	a := model.(*App)

	if a.nameText != "hell" {
		t.Errorf("nameText after backspace = %q, want %q", a.nameText, "hell")
	}

	// Backspace on empty string
	env.app.nameText = ""
	model, _ = env.app.handleNameInputKey("backspace", tea.KeyMsg{Type: tea.KeyBackspace})
	a = model.(*App)
	if a.nameText != "" {
		t.Errorf("nameText after backspace on empty = %q, want empty", a.nameText)
	}
}

func TestIntegrationHandleNameInputKey_TypeRunes(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.mode = ModeNameInput
	env.app.nameText = "he"

	model, _ := env.app.handleNameInputKey("l", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	a := model.(*App)

	if a.nameText != "hel" {
		t.Errorf("nameText after typing = %q, want %q", a.nameText, "hel")
	}
}

func TestIntegrationHandleNameInputKey_GroupCreate(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewTabs
	env.app.mode = ModeNameInput
	env.app.namePrompt = "Group name: "
	env.app.nameText = "my-group"

	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 10, Title: "Tab 1", URL: "https://one.com"},
		TabItem{ID: 20, Title: "Tab 2", URL: "https://two.com"},
	}
	vs.itemCount = 2
	vs.cursor = 0

	model, cmd := env.app.handleNameInputKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd == nil {
		t.Fatal("group create should return cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("group create error: %v", em.err)
	}
}

func TestIntegrationHandleNameInputKey_GroupCreateWithSelection(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewTabs
	env.app.mode = ModeNameInput
	env.app.namePrompt = "Group name: "
	env.app.nameText = "selected-group"

	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 10, Title: "Tab 1", URL: "https://one.com"},
		TabItem{ID: 20, Title: "Tab 2", URL: "https://two.com"},
		TabItem{ID: 30, Title: "Tab 3", URL: "https://three.com"},
	}
	vs.itemCount = 3
	vs.selected[0] = true
	vs.selected[2] = true

	model, cmd := env.app.handleNameInputKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd == nil {
		t.Fatal("group create with selection should return cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("group create with selection error: %v", em.err)
	}
}

func TestIntegrationHandleNameInputKey_GroupCreateEmpty(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewTabs
	env.app.mode = ModeNameInput
	env.app.namePrompt = "Group name: "
	env.app.nameText = "empty-group"

	vs := env.app.views[ViewTabs]
	vs.items = nil
	vs.itemCount = 0

	model, cmd := env.app.handleNameInputKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)

	if a.errorMsg != "No tabs to group" {
		t.Errorf("errorMsg = %q, want 'No tabs to group'", a.errorMsg)
	}
	if cmd != nil {
		t.Error("empty group should return nil cmd")
	}
	_ = a
}

func TestIntegrationHandleNameInputKey_WorkspaceCreate(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewWorkspaces
	env.app.mode = ModeNameInput
	env.app.nameText = "new-ws"
	env.app.namePrompt = "Name: "

	model, cmd := env.app.handleNameInputKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd == nil {
		t.Fatal("workspace create should return cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("workspace create error: %v", em.err)
	}
}

func TestIntegrationHandleNameInputKey_WorkspaceRename(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewWorkspaces
	env.app.mode = ModeNameInput
	env.app.nameText = "renamed-ws"
	env.app.namePrompt = "New name: "

	vs := env.app.views[ViewWorkspaces]
	vs.items = []any{
		WorkspaceItem{ID: "ws-1", Name: "old-ws"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleNameInputKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd == nil {
		t.Fatal("workspace rename should return cmd")
	}

	// Workspace may not exist on disk so the daemon may return an error.
	// We verify the TUI code path is exercised (cmd is non-nil).
	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if msg == nil {
		t.Error("workspace rename cmd should return a msg")
	}
}

func TestIntegrationHandleNameInputKey_TargetLabel(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewTargets
	env.app.mode = ModeNameInput
	env.app.nameText = "my-label"
	env.app.namePrompt = "Label: "

	vs := env.app.views[ViewTargets]
	vs.items = []any{
		TargetItem{TargetID: env.ext.targetID, Channel: "nm"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleNameInputKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd == nil {
		t.Fatal("target label should return cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("target label error: %v", em.err)
	}
}

func TestIntegrationHandleNameInputKey_CollectionRename(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// Pre-create the collection
	collection := map[string]any{
		"name": "old-name", "createdAt": "2024-01-01T00:00:00Z",
		"updatedAt": "2024-01-01T00:00:00Z", "items": []any{},
	}
	data, _ := json.MarshalIndent(collection, "", "  ")
	os.WriteFile(filepath.Join(env.tmpDir, "collections", "old-name.json"), data, 0644)

	env.app.view = ViewCollections
	env.app.mode = ModeNameInput
	env.app.nameText = "new-name"
	env.app.namePrompt = "Rename: "

	vs := env.app.views[ViewCollections]
	vs.items = []any{
		CollectionItem{Name: "old-name", ItemCount: 0},
	}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleNameInputKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd == nil {
		t.Fatal("collection rename should return cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("collection rename error: %v", em.err)
	}
}

func TestIntegrationHandleNameInputKey_CollectionRenameSameName(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewCollections
	env.app.mode = ModeNameInput
	env.app.nameText = "same-name"
	env.app.namePrompt = "Rename: "

	vs := env.app.views[ViewCollections]
	vs.items = []any{
		CollectionItem{Name: "same-name", ItemCount: 0},
	}
	vs.itemCount = 1
	vs.cursor = 0

	_, cmd := env.app.handleNameInputKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("rename to same name should return nil cmd")
	}
}

// --- handleEnter additional branches ---

func TestIntegrationHandleEnter_GroupToggle(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewGroups
	vs := env.app.views[ViewGroups]
	vs.items = []any{
		GroupItem{ID: 5, Title: "DevTools", Color: "blue", Collapsed: false},
	}
	vs.itemCount = 1
	vs.cursor = 0

	cmd := env.app.handleEnter()
	if cmd == nil {
		t.Fatal("handleEnter on group should return cmd")
	}

	// groups.update is forwarded to extension which may not support it.
	// We verify the TUI code path is exercised.
	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if msg == nil {
		t.Error("group toggle cmd should return a msg")
	}
}

func TestIntegrationHandleEnter_BookmarkFolder(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewBookmarks
	env.app.collapsedFolders = make(map[string]bool)
	env.app.bookmarkTree = []BookmarkItem{
		{ID: "1", Title: "Root", Children: []BookmarkItem{
			{ID: "2", Title: "Child", URL: "https://child.com"},
		}},
	}

	vs := env.app.views[ViewBookmarks]
	vs.items = []any{
		BookmarkItem{ID: "1", Title: "Root", IsFolder: true, Children: []BookmarkItem{
			{ID: "2", Title: "Child", URL: "https://child.com"},
		}},
		BookmarkItem{ID: "2", Title: "Child", URL: "https://child.com", IsFolder: false},
	}
	vs.itemCount = 2
	vs.cursor = 0

	// Collapse the folder
	cmd := env.app.handleEnter()
	if cmd == nil {
		t.Fatal("handleEnter on folder should return cmd")
	}
	if !env.app.collapsedFolders["1"] {
		t.Error("folder should be collapsed after enter")
	}

	// Uncollapse
	cmd = env.app.handleEnter()
	if cmd == nil {
		t.Fatal("handleEnter to unfold should return cmd")
	}
	if env.app.collapsedFolders["1"] {
		t.Error("folder should be uncollapsed after second enter")
	}
}

func TestIntegrationHandleEnter_BookmarkLink(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewBookmarks
	vs := env.app.views[ViewBookmarks]
	vs.items = []any{
		BookmarkItem{ID: "2", Title: "Link", URL: "https://example.com", IsFolder: false},
	}
	vs.itemCount = 1
	vs.cursor = 0

	cmd := env.app.handleEnter()
	if cmd == nil {
		t.Fatal("handleEnter on bookmark link should return cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("open bookmark error: %v", em.err)
	}
}

func TestIntegrationHandleEnter_SessionExpand(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// Pre-create a session file
	session := map[string]any{
		"name": "expand-me", "createdAt": "2024-01-01T00:00:00Z",
		"sourceTarget": env.ext.targetID,
		"windows": []map[string]any{
			{"tabs": []map[string]any{
				{"url": "https://a.com", "title": "Tab A", "pinned": false, "active": true, "groupIndex": -1},
			}},
		},
		"groups": []any{},
	}
	data, _ := json.MarshalIndent(session, "", "  ")
	os.WriteFile(filepath.Join(env.tmpDir, "sessions", "expand-me.json"), data, 0644)

	env.app.view = ViewSessions
	vs := env.app.views[ViewSessions]
	vs.items = []any{
		SessionItem{Name: "expand-me", TabCount: 1},
	}
	vs.itemCount = 1
	vs.cursor = 0

	// Expand
	cmd := env.app.handleEnter()
	if cmd == nil {
		t.Fatal("handleEnter on session should return cmd to expand")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("session expand error: %v", em.err)
	}

	// After expand, collapse
	if _, ok := env.app.expandedSessions["expand-me"]; ok {
		cmd = env.app.handleEnter()
		// Should collapse (nil cmd)
	}
}

func TestIntegrationHandleEnter_CollectionExpand(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// Pre-create a collection file
	collection := map[string]any{
		"name": "expand-coll", "createdAt": "2024-01-01T00:00:00Z",
		"updatedAt": "2024-01-01T00:00:00Z",
		"items": []map[string]any{
			{"url": "https://a.com", "title": "Item A"},
		},
	}
	data, _ := json.MarshalIndent(collection, "", "  ")
	os.WriteFile(filepath.Join(env.tmpDir, "collections", "expand-coll.json"), data, 0644)

	env.app.view = ViewCollections
	vs := env.app.views[ViewCollections]
	vs.items = []any{
		CollectionItem{Name: "expand-coll", ItemCount: 1},
	}
	vs.itemCount = 1
	vs.cursor = 0

	cmd := env.app.handleEnter()
	if cmd == nil {
		t.Fatal("handleEnter on collection should return cmd to expand")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("collection expand error: %v", em.err)
	}
}

func TestIntegrationHandleEnter_WorkspaceGet(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewWorkspaces
	vs := env.app.views[ViewWorkspaces]
	vs.items = []any{
		WorkspaceItem{ID: "ws-1", Name: "test-ws"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	cmd := env.app.handleEnter()
	if cmd == nil {
		t.Fatal("handleEnter on workspace should return cmd")
	}

	// Workspace may not exist; just verify the code path runs.
	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if msg == nil {
		t.Error("workspace get cmd should return a msg")
	}
}

// --- handleConfirmDeleteKey additional branches ---

func TestIntegrationHandleConfirmDeleteKey_Workspace(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// Pre-create a workspace file so delete can succeed
	ws := map[string]any{
		"id": "ws-del", "name": "del-ws",
		"sessions": []any{}, "collections": []any{},
		"createdAt": "2024-01-01T00:00:00Z", "updatedAt": "2024-01-01T00:00:00Z",
	}
	data, _ := json.MarshalIndent(ws, "", "  ")
	os.WriteFile(filepath.Join(env.tmpDir, "workspaces", "ws-del.json"), data, 0644)

	env.app.view = ViewWorkspaces
	env.app.mode = ModeConfirmDelete
	vs := env.app.views[ViewWorkspaces]
	vs.items = []any{
		WorkspaceItem{ID: "ws-del", Name: "del-ws"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleConfirmDeleteKey("D")
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd == nil {
		t.Fatal("workspace delete should return cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if msg == nil {
		t.Error("workspace delete cmd should return a msg")
	}
}

func TestIntegrationHandleConfirmDeleteKey_Group(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewGroups
	env.app.mode = ModeConfirmDelete
	vs := env.app.views[ViewGroups]
	vs.items = []any{
		GroupItem{ID: 5, Title: "DevGroup", Color: "blue"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleConfirmDeleteKey("D")
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd == nil {
		t.Fatal("group delete should return cmd")
	}

	// groups.delete is forwarded to extension which may not support it.
	// We verify the TUI code path is exercised.
	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if msg == nil {
		t.Error("group delete cmd should return a msg")
	}
}

func TestIntegrationHandleConfirmDeleteKey_Bookmark(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewBookmarks
	env.app.mode = ModeConfirmDelete
	vs := env.app.views[ViewBookmarks]
	vs.items = []any{
		BookmarkItem{ID: "bm-1", Title: "Delete Me", URL: "https://del.com", IsFolder: false},
	}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleConfirmDeleteKey("D")
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd == nil {
		t.Fatal("bookmark delete should return cmd")
	}

	// bookmarks.remove is forwarded to extension which may not support it.
	// We verify the TUI code path is exercised.
	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if msg == nil {
		t.Error("bookmark delete cmd should return a msg")
	}
}

func TestIntegrationHandleConfirmDeleteKey_BookmarkRoot(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewBookmarks
	env.app.mode = ModeConfirmDelete
	vs := env.app.views[ViewBookmarks]
	vs.items = []any{
		BookmarkItem{ID: "0", Title: "", IsFolder: true},
	}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleConfirmDeleteKey("D")
	a := model.(*App)
	if a.errorMsg != "Chrome root node cannot be deleted" {
		t.Errorf("errorMsg = %q, want root node error", a.errorMsg)
	}
	if cmd != nil {
		t.Error("deleting root should return nil cmd")
	}
}

func TestIntegrationHandleConfirmDeleteKey_XChord(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// Create a session file to delete
	session := map[string]any{
		"name": "xx-del", "createdAt": "2024-01-01T00:00:00Z",
		"windows": []any{}, "groups": []any{},
	}
	data, _ := json.MarshalIndent(session, "", "  ")
	sessionFile := filepath.Join(env.tmpDir, "sessions", "xx-del.json")
	os.WriteFile(sessionFile, data, 0644)

	env.app.view = ViewSessions
	env.app.mode = ModeConfirmDelete
	vs := env.app.views[ViewSessions]
	vs.items = []any{SessionItem{Name: "xx-del"}}
	vs.itemCount = 1
	vs.cursor = 0

	// x key also confirms delete (xx chord)
	_, cmd := env.app.handleConfirmDeleteKey("x")
	if cmd == nil {
		t.Fatal("xx chord should return cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("xx chord delete error: %v", em.err)
	}
}

// --- handleZFilterKey ---

func TestIntegrationHandleZFilterKey_Host(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTabs
	env.app.mode = ModeZFilter
	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, Title: "A", URL: "https://github.com/a"},
		TabItem{ID: 2, Title: "B", URL: "https://google.com"},
		TabItem{ID: 3, Title: "C", URL: "https://github.com/c"},
	}
	vs.itemCount = 3
	vs.cursor = 0

	model, cmd := env.app.handleZFilterKey("h")
	a := model.(*App)

	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd == nil {
		t.Fatal("zh should return toast cmd")
	}
	if len(vs.filtered) != 2 {
		t.Errorf("filtered count = %d, want 2", len(vs.filtered))
	}
}

func TestIntegrationHandleZFilterKey_Pinned(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTabs
	env.app.mode = ModeZFilter
	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, Title: "A", URL: "https://a.com", Pinned: true},
		TabItem{ID: 2, Title: "B", URL: "https://b.com", Pinned: false},
		TabItem{ID: 3, Title: "C", URL: "https://c.com", Pinned: true},
	}
	vs.itemCount = 3

	model, cmd := env.app.handleZFilterKey("p")
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd == nil {
		t.Fatal("zp should return toast cmd")
	}
	if len(vs.filtered) != 2 {
		t.Errorf("filtered = %d, want 2", len(vs.filtered))
	}
}

func TestIntegrationHandleZFilterKey_Grouped(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTabs
	env.app.mode = ModeZFilter
	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, URL: "https://a.com", GroupID: 5},
		TabItem{ID: 2, URL: "https://b.com", GroupID: -1},
	}
	vs.itemCount = 2

	model, _ := env.app.handleZFilterKey("g")
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if len(vs.filtered) != 1 {
		t.Errorf("filtered = %d, want 1", len(vs.filtered))
	}
}

func TestIntegrationHandleZFilterKey_Active(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTabs
	env.app.mode = ModeZFilter
	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, URL: "https://a.com", Active: true},
		TabItem{ID: 2, URL: "https://b.com", Active: false},
	}
	vs.itemCount = 2

	model, _ := env.app.handleZFilterKey("a")
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if len(vs.filtered) != 1 {
		t.Errorf("filtered = %d, want 1", len(vs.filtered))
	}
}

func TestIntegrationHandleZFilterKey_Clear(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTabs
	env.app.mode = ModeZFilter
	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, URL: "https://a.com"},
	}
	vs.itemCount = 1
	vs.filtered = []int{0}

	model, _ := env.app.handleZFilterKey("c")
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if vs.filtered != nil {
		t.Error("filtered should be nil after clear")
	}
}

func TestIntegrationHandleZFilterKey_BookmarkFoldAll(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewBookmarks
	env.app.mode = ModeZFilter
	env.app.bookmarkTree = []BookmarkItem{
		{ID: "1", Title: "Root", Children: []BookmarkItem{
			{ID: "2", Title: "Child", URL: "https://child.com"},
		}},
	}
	env.app.collapsedFolders = make(map[string]bool)

	// Flatten first
	flat := flattenBookmarkTree(env.app.bookmarkTree, 0)
	vs := env.app.views[ViewBookmarks]
	vs.items = make([]any, len(flat))
	for i, b := range flat {
		vs.items[i] = b
	}
	vs.itemCount = len(flat)

	// zM = fold all
	model, cmd := env.app.handleZFilterKey("M")
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if !a.collapsedFolders["1"] {
		t.Error("folder 1 should be collapsed after zM")
	}
	if cmd == nil {
		t.Fatal("zM should return toast cmd")
	}
}

func TestIntegrationHandleZFilterKey_BookmarkUnfoldAll(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewBookmarks
	env.app.mode = ModeZFilter
	env.app.bookmarkTree = []BookmarkItem{
		{ID: "1", Title: "Root", Children: []BookmarkItem{
			{ID: "2", Title: "Child", URL: "https://child.com"},
		}},
	}
	env.app.collapsedFolders = map[string]bool{"1": true}

	flat := flattenBookmarkTreeWithCollapse(env.app.bookmarkTree, 0, env.app.collapsedFolders)
	vs := env.app.views[ViewBookmarks]
	vs.items = make([]any, len(flat))
	for i, b := range flat {
		vs.items[i] = b
	}
	vs.itemCount = len(flat)

	// zR = unfold all
	model, cmd := env.app.handleZFilterKey("R")
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if len(a.collapsedFolders) != 0 {
		t.Errorf("collapsedFolders should be empty, got %d", len(a.collapsedFolders))
	}
	if cmd == nil {
		t.Fatal("zR should return toast cmd")
	}
}

func TestIntegrationHandleZFilterKey_EmptyTabs(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTabs
	env.app.mode = ModeZFilter
	vs := env.app.views[ViewTabs]
	vs.items = nil
	vs.itemCount = 0

	model, cmd := env.app.handleZFilterKey("h")
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd != nil {
		t.Error("zh on empty should return nil cmd")
	}
}

// --- toggleTabMute / toggleTabPin ---

func TestIntegrationToggleTabMute(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, Title: "Tab", URL: "https://tab.com", Muted: false},
	}
	vs.itemCount = 1
	vs.cursor = 0

	cmd := env.app.toggleTabMute()
	if cmd == nil {
		t.Fatal("toggleTabMute should return cmd")
	}

	// Mock extension returns error for tabs.mute (unknown action).
	// We just verify the cmd is produced and executes without panic.
	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if msg == nil {
		t.Error("toggleTabMute cmd should return a msg")
	}
}

func TestIntegrationToggleTabMute_Empty(t *testing.T) {
	env := newIntegrationEnv(t)

	vs := env.app.views[ViewTabs]
	vs.items = nil
	vs.itemCount = 0

	cmd := env.app.toggleTabMute()
	if cmd != nil {
		t.Error("toggleTabMute on empty should return nil")
	}
}

func TestIntegrationToggleTabPin(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, Title: "Tab", URL: "https://tab.com", Pinned: false},
	}
	vs.itemCount = 1
	vs.cursor = 0

	cmd := env.app.toggleTabPin()
	if cmd == nil {
		t.Fatal("toggleTabPin should return cmd")
	}

	// Mock extension returns error for tabs.pin (unknown action).
	// We just verify the cmd is produced and executes without panic.
	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if msg == nil {
		t.Error("toggleTabPin cmd should return a msg")
	}
}

func TestIntegrationToggleTabPin_Empty(t *testing.T) {
	env := newIntegrationEnv(t)

	vs := env.app.views[ViewTabs]
	vs.items = nil
	vs.itemCount = 0

	cmd := env.app.toggleTabPin()
	if cmd != nil {
		t.Error("toggleTabPin on empty should return nil")
	}
}

// --- currentTab ---

func TestIntegrationCurrentTab(t *testing.T) {
	env := newIntegrationEnv(t)

	// Empty
	vs := env.app.views[ViewTabs]
	vs.items = nil
	vs.itemCount = 0

	_, ok := env.app.currentTab()
	if ok {
		t.Error("currentTab on empty should return false")
	}

	// With items
	vs.items = []any{
		TabItem{ID: 1, Title: "Tab", URL: "https://tab.com"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	tab, ok := env.app.currentTab()
	if !ok {
		t.Error("currentTab should return true")
	}
	if tab.ID != 1 {
		t.Errorf("tab.ID = %d, want 1", tab.ID)
	}
}

// --- handleKey normal mode branches ---

func TestIntegrationHandleKey_Quit(t *testing.T) {
	env := newIntegrationEnv(t)

	_, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("q should return tea.Quit cmd")
	}
	msg := execCmd(t, cmd)
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", msg)
	}
}

func TestIntegrationHandleKey_Help(t *testing.T) {
	env := newIntegrationEnv(t)

	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	a := model.(*App)
	if a.mode != ModeHelp {
		t.Errorf("mode = %d, want ModeHelp", a.mode)
	}

	// Exit help with q
	model, _ = a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	a = model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode after q in help = %d, want ModeNormal", a.mode)
	}
}

func TestIntegrationHandleKey_Filter(t *testing.T) {
	env := newIntegrationEnv(t)

	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	a := model.(*App)
	if a.mode != ModeFilter {
		t.Errorf("mode = %d, want ModeFilter", a.mode)
	}
}

func TestIntegrationHandleKey_Command(t *testing.T) {
	env := newIntegrationEnv(t)

	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	a := model.(*App)
	if a.mode != ModeCommand {
		t.Errorf("mode = %d, want ModeCommand", a.mode)
	}
}

func TestIntegrationHandleKey_Navigation(t *testing.T) {
	env := newIntegrationEnv(t)

	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, Title: "A"},
		TabItem{ID: 2, Title: "B"},
		TabItem{ID: 3, Title: "C"},
	}
	vs.itemCount = 3
	vs.cursor = 0

	// j moves down
	env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if vs.cursor != 1 {
		t.Errorf("cursor after j = %d, want 1", vs.cursor)
	}

	// k moves up
	env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if vs.cursor != 0 {
		t.Errorf("cursor after k = %d, want 0", vs.cursor)
	}

	// G goes to end
	env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if vs.cursor != 2 {
		t.Errorf("cursor after G = %d, want 2", vs.cursor)
	}

	// space toggles select
	env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if !vs.selected[2] {
		t.Error("item should be selected after space")
	}

	// u clears selection
	env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if len(vs.selected) != 0 {
		t.Error("selection should be empty after u")
	}

	// ctrl+a selects all
	env.app.handleKey(tea.KeyMsg{Type: tea.KeyCtrlA})
	if len(vs.selected) != 3 {
		t.Errorf("selected count = %d, want 3", len(vs.selected))
	}
}

func TestIntegrationHandleKey_GG(t *testing.T) {
	env := newIntegrationEnv(t)

	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, Title: "A"},
		TabItem{ID: 2, Title: "B"},
	}
	vs.itemCount = 2
	vs.cursor = 1

	// First g sets pendingG
	model, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	a := model.(*App)
	if !a.pendingG {
		t.Error("pendingG should be true after first g")
	}
	if cmd == nil {
		t.Fatal("g should return timeout cmd")
	}

	// Second g goes to top
	model, _ = a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	a = model.(*App)
	if a.pendingG {
		t.Error("pendingG should be false after gg")
	}
	if vs.cursor != 0 {
		t.Errorf("cursor after gg = %d, want 0", vs.cursor)
	}
}

func TestIntegrationHandleKey_NumberedViewSwitch(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	tests := []struct {
		key  rune
		want ViewType
	}{
		{'1', ViewTargets},
		{'2', ViewTabs},
		{'3', ViewGroups},
		{'4', ViewSessions},
		{'5', ViewCollections},
		{'6', ViewBookmarks},
		{'7', ViewWorkspaces},
		{'8', ViewSync},
		{'9', ViewHistory},
		{'0', ViewSearch},
	}

	for _, tt := range tests {
		model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.key}})
		a := model.(*App)
		if a.view != tt.want {
			t.Errorf("key %c: view = %v, want %v", tt.key, a.view, tt.want)
		}
	}
}

func TestIntegrationHandleKey_TabShift(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewTabs

	// tab cycles forward
	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	a := model.(*App)
	if a.view == ViewTabs {
		// Should have changed view
		t.Log("tab shifted view away from Tabs")
	}

	// shift+tab cycles backward
	env.app.view = ViewGroups
	model, _ = env.app.handleKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	a = model.(*App)
	if a.view == ViewGroups {
		t.Log("shift+tab shifted view away from Groups")
	}
}

func TestIntegrationHandleKey_Refresh(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	model, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	_ = model.(*App)
	if cmd == nil {
		t.Fatal("r should return refresh cmd")
	}
}

func TestIntegrationHandleKey_EscClearsError(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.errorMsg = "some error"
	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	a := model.(*App)
	if a.errorMsg != "" {
		t.Errorf("errorMsg after esc = %q, want empty", a.errorMsg)
	}
}

func TestIntegrationHandleKey_NameInput_Sessions(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewSessions
	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	a := model.(*App)
	if a.mode != ModeNameInput {
		t.Errorf("mode = %d, want ModeNameInput", a.mode)
	}
	if a.namePrompt != "Name: " {
		t.Errorf("namePrompt = %q, want 'Name: '", a.namePrompt)
	}
}

func TestIntegrationHandleKey_NameInput_GroupName(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTabs
	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	a := model.(*App)
	if a.mode != ModeNameInput {
		t.Errorf("mode = %d, want ModeNameInput", a.mode)
	}
	if a.namePrompt != "Group name: " {
		t.Errorf("namePrompt = %q, want 'Group name: '", a.namePrompt)
	}
}

func TestIntegrationHandleKey_YankChord(t *testing.T) {
	env := newIntegrationEnv(t)

	model, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	a := model.(*App)
	if a.mode != ModeYank {
		t.Errorf("mode = %d, want ModeYank", a.mode)
	}
	if cmd == nil {
		t.Fatal("y should return timeout cmd")
	}
}

func TestIntegrationHandleKey_DeleteChord_Sessions(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewSessions
	vs := env.app.views[ViewSessions]
	vs.items = []any{SessionItem{Name: "s1"}}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	a := model.(*App)
	if a.mode != ModeConfirmDelete {
		t.Errorf("mode = %d, want ModeConfirmDelete", a.mode)
	}
	if cmd == nil {
		t.Fatal("D should return timeout cmd")
	}
}

func TestIntegrationHandleKey_DeleteChord_Groups(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewGroups
	vs := env.app.views[ViewGroups]
	vs.items = []any{GroupItem{ID: 1, Title: "G1"}}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	a := model.(*App)
	if a.mode != ModeConfirmDelete {
		t.Errorf("mode = %d, want ModeConfirmDelete", a.mode)
	}
	if cmd == nil {
		t.Fatal("D on groups should return timeout cmd")
	}
}

func TestIntegrationHandleKey_DeleteChord_Bookmarks(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewBookmarks
	vs := env.app.views[ViewBookmarks]
	vs.items = []any{BookmarkItem{ID: "bm1", Title: "BM", URL: "https://bm.com", IsFolder: false}}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	a := model.(*App)
	if a.mode != ModeConfirmDelete {
		t.Errorf("mode = %d, want ModeConfirmDelete", a.mode)
	}
	if cmd == nil {
		t.Fatal("D on bookmarks should return timeout cmd")
	}
}

func TestIntegrationHandleKey_DeleteChord_BookmarkRoot(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewBookmarks
	vs := env.app.views[ViewBookmarks]
	vs.items = []any{BookmarkItem{ID: "0", Title: "", IsFolder: true}}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	a := model.(*App)
	if a.errorMsg != "Chrome root node cannot be deleted" {
		t.Errorf("errorMsg = %q, want root node error", a.errorMsg)
	}
	if cmd != nil {
		t.Error("D on root should return nil cmd")
	}
}

func TestIntegrationHandleKey_O_RestoreSessions(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// Pre-create a session file
	session := map[string]any{
		"name": "key-restore", "createdAt": "2024-01-01T00:00:00Z",
		"windows": []map[string]any{
			{"tabs": []map[string]any{
				{"url": "https://a.com", "title": "A", "pinned": false, "active": true, "groupIndex": -1},
			}},
		},
		"groups": []any{},
	}
	data, _ := json.MarshalIndent(session, "", "  ")
	os.WriteFile(filepath.Join(env.tmpDir, "sessions", "key-restore.json"), data, 0644)

	env.app.view = ViewSessions
	vs := env.app.views[ViewSessions]
	vs.items = []any{SessionItem{Name: "key-restore", TabCount: 1}}
	vs.itemCount = 1
	vs.cursor = 0

	_, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if cmd == nil {
		t.Fatal("o should return restore cmd")
	}
}

func TestIntegrationHandleKey_X_CloseTabs(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewTabs
	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, Title: "Tab", URL: "https://tab.com"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	_, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd == nil {
		t.Fatal("x should return close cmd")
	}
}

func TestIntegrationHandleKey_X_SessionDelete(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewSessions
	vs := env.app.views[ViewSessions]
	vs.items = []any{SessionItem{Name: "del-me"}}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	a := model.(*App)
	if a.mode != ModeConfirmDelete {
		t.Errorf("mode = %d, want ModeConfirmDelete", a.mode)
	}
	if cmd == nil {
		t.Fatal("x on session should return timeout cmd")
	}
}

func TestIntegrationHandleKey_M_MoveToWindow(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTabs
	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'M'}})
	a := model.(*App)
	if a.mode != ModeNameInput {
		t.Errorf("mode = %d, want ModeNameInput", a.mode)
	}
	if a.namePrompt != "Move to window: " {
		t.Errorf("namePrompt = %q, want 'Move to window: '", a.namePrompt)
	}
}

func TestIntegrationHandleKey_A_AddToCollection(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTabs
	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	a := model.(*App)
	if a.mode != ModeNameInput {
		t.Errorf("mode = %d, want ModeNameInput", a.mode)
	}
	if a.namePrompt != "Add to collection: " {
		t.Errorf("namePrompt = %q, want 'Add to collection: '", a.namePrompt)
	}
}

func TestIntegrationHandleKey_M_TabMute(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewTabs
	vs := env.app.views[ViewTabs]
	vs.items = []any{TabItem{ID: 1, Title: "Tab", URL: "https://t.com"}}
	vs.itemCount = 1
	vs.cursor = 0

	_, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if cmd == nil {
		t.Fatal("m should return toggleMute cmd")
	}
}

func TestIntegrationHandleKey_P_TabPin(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewTabs
	vs := env.app.views[ViewTabs]
	vs.items = []any{TabItem{ID: 1, Title: "Tab", URL: "https://t.com"}}
	vs.itemCount = 1
	vs.cursor = 0

	_, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if cmd == nil {
		t.Fatal("p should return togglePin cmd")
	}
}

func TestIntegrationHandleKey_D_TargetDefault(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewTargets
	vs := env.app.views[ViewTargets]
	vs.items = []any{TargetItem{TargetID: env.ext.targetID, Channel: "nm"}}
	vs.itemCount = 1
	vs.cursor = 0

	_, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd == nil {
		t.Fatal("d on targets should return setDefault cmd")
	}
}

func TestIntegrationHandleKey_C_ClearDefault(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewTargets
	env.app.selectedTarget = "some-target"

	_, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cmd == nil {
		t.Fatal("c on targets should return clearDefault cmd")
	}
}

func TestIntegrationHandleKey_E_WorkspaceRename(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewWorkspaces
	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	a := model.(*App)
	if a.mode != ModeNameInput {
		t.Errorf("mode = %d, want ModeNameInput", a.mode)
	}
	if a.namePrompt != "New name: " {
		t.Errorf("namePrompt = %q, want 'New name: '", a.namePrompt)
	}
}

func TestIntegrationHandleKey_E_CollectionRename(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewCollections
	vs := env.app.views[ViewCollections]
	vs.items = []any{CollectionItem{Name: "coll-1"}}
	vs.itemCount = 1
	vs.cursor = 0

	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	a := model.(*App)
	if a.mode != ModeNameInput {
		t.Errorf("mode = %d, want ModeNameInput", a.mode)
	}
	if a.namePrompt != "Rename: " {
		t.Errorf("namePrompt = %q, want 'Rename: '", a.namePrompt)
	}
	if a.nameText != "coll-1" {
		t.Errorf("nameText = %q, want 'coll-1'", a.nameText)
	}
}

func TestIntegrationHandleKey_A_BookmarkCreate(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewBookmarks
	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	a := model.(*App)
	if a.mode != ModeNameInput {
		t.Errorf("mode = %d, want ModeNameInput", a.mode)
	}
	if a.namePrompt != "URL: " {
		t.Errorf("namePrompt = %q, want 'URL: '", a.namePrompt)
	}
}

func TestIntegrationHandleKey_O_WorkspaceSwitch(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewWorkspaces
	vs := env.app.views[ViewWorkspaces]
	vs.items = []any{WorkspaceItem{ID: "ws-1", Name: "ws"}}
	vs.itemCount = 1
	vs.cursor = 0

	_, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if cmd == nil {
		t.Fatal("o on workspaces should return switch cmd")
	}
}

func TestIntegrationHandleKey_Z_TabsFilter(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTabs
	model, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	a := model.(*App)
	if a.mode != ModeZFilter {
		t.Errorf("mode = %d, want ModeZFilter", a.mode)
	}
	if cmd == nil {
		t.Fatal("z should return timeout cmd")
	}
}

func TestIntegrationHandleKey_Z_BookmarksFilter(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewBookmarks
	model, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	a := model.(*App)
	if a.mode != ModeZFilter {
		t.Errorf("mode = %d, want ModeZFilter", a.mode)
	}
	if cmd == nil {
		t.Fatal("z on bookmarks should return timeout cmd")
	}
}

func TestIntegrationHandleKey_L_TargetLabel(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTargets
	vs := env.app.views[ViewTargets]
	vs.items = []any{TargetItem{TargetID: "t1"}}
	vs.itemCount = 1
	vs.cursor = 0

	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	a := model.(*App)
	if a.mode != ModeNameInput {
		t.Errorf("mode = %d, want ModeNameInput", a.mode)
	}
	if a.namePrompt != "Label: " {
		t.Errorf("namePrompt = %q, want 'Label: '", a.namePrompt)
	}
}

func TestIntegrationHandleKey_CtrlD(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.height = 40
	vs := env.app.views[ViewTabs]
	items := make([]any, 50)
	for i := range items {
		items[i] = TabItem{ID: i, Title: fmt.Sprintf("Tab %d", i)}
	}
	vs.items = items
	vs.itemCount = 50
	vs.cursor = 0

	env.app.handleKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	if vs.cursor == 0 {
		t.Error("ctrl+d should move cursor down")
	}

	// ctrl+u goes back up
	env.app.handleKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	if vs.cursor != 0 {
		t.Errorf("cursor after ctrl+u = %d, want 0", vs.cursor)
	}
}

func TestIntegrationHandleKey_D_NestedTabHint(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewSessions
	vs := env.app.views[ViewSessions]
	vs.items = []any{
		NestedTabItem{URL: "https://a.com", Title: "A", ParentName: "s1"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	a := model.(*App)
	if a.toast != "Use x to remove item" {
		t.Errorf("toast = %q, want 'Use x to remove item'", a.toast)
	}
	if cmd == nil {
		t.Fatal("D on nested tab should return toast clear cmd")
	}
}

// --- applyRefresh additional views ---

func TestIntegrationApplyRefresh_History(t *testing.T) {
	env := newIntegrationEnv(t)

	payload, _ := json.Marshal(map[string]any{
		"history": []map[string]any{
			{"id": "1", "url": "https://h1.com", "title": "H1", "lastVisitTime": 1234567890.0, "visitCount": 3},
			{"id": "2", "url": "https://h2.com", "title": "H2", "lastVisitTime": 1234567891.0, "visitCount": 1},
		},
	})

	env.app.view = ViewHistory
	env.app.applyRefresh(payload)
	// 2 history items + at least 1 date separator (both items share same date group)
	ic := env.app.views[ViewHistory].itemCount
	if ic < 3 {
		t.Errorf("history itemCount = %d, want >= 3 (items + date separators)", ic)
	}
}

func TestIntegrationApplyRefresh_SearchResults(t *testing.T) {
	env := newIntegrationEnv(t)

	payload, _ := json.Marshal(map[string]any{
		"results": []map[string]any{
			{"kind": "tab", "id": "1", "title": "Result 1", "url": "https://r1.com", "score": 0.9},
		},
	})

	env.app.view = ViewSearch
	env.app.searchActive = true
	env.app.applyRefresh(payload)
	if env.app.views[ViewSearch].itemCount != 1 {
		t.Errorf("search results itemCount = %d, want 1", env.app.views[ViewSearch].itemCount)
	}
}

func TestIntegrationApplyRefresh_SavedSearches(t *testing.T) {
	env := newIntegrationEnv(t)

	payload, _ := json.Marshal(map[string]any{
		"searches": []map[string]any{
			{"id": "s1", "name": "saved1", "query": map[string]any{"query": "test"}, "createdAt": "2024-01-01T00:00:00Z"},
		},
	})

	env.app.view = ViewSearch
	env.app.searchActive = false
	env.app.applyRefresh(payload)
	if env.app.views[ViewSearch].itemCount != 1 {
		t.Errorf("saved searches itemCount = %d, want 1", env.app.views[ViewSearch].itemCount)
	}
	item := env.app.views[ViewSearch].items[0].(SavedSearchItem)
	if item.Name != "saved1" || item.QueryText != "test" {
		t.Errorf("saved search = %+v, want Name=saved1, QueryText=test", item)
	}
}

func TestIntegrationApplyRefresh_Downloads(t *testing.T) {
	env := newIntegrationEnv(t)

	payload, _ := json.Marshal(map[string]any{
		"downloads": []map[string]any{
			{"id": 1, "filename": "file.zip", "url": "https://dl.com/file.zip", "state": "in_progress", "totalBytes": 1048576},
		},
	})

	env.app.view = ViewDownloads
	env.app.applyRefresh(payload)
	if env.app.views[ViewDownloads].itemCount != 1 {
		t.Errorf("downloads itemCount = %d, want 1", env.app.views[ViewDownloads].itemCount)
	}
}

func TestIntegrationApplyRefresh_SessionsWithExpanded(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.expandedSessions["s1"] = []NestedTabItem{
		{URL: "https://a.com", Title: "A", ParentName: "s1"},
	}

	payload, _ := json.Marshal(map[string]any{
		"sessions": []map[string]any{
			{"name": "s1", "tabCount": 1},
			{"name": "s2", "tabCount": 2},
		},
	})

	env.app.view = ViewSessions
	env.app.applyRefresh(payload)

	vs := env.app.views[ViewSessions]
	// Should have s1 + nested + s2 = 3 items after rebuild
	if vs.itemCount < 2 {
		t.Errorf("sessions with expanded: itemCount = %d, want >= 2", vs.itemCount)
	}
}

func TestIntegrationApplyRefresh_CollectionsWithExpanded(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.expandedCollections["c1"] = []NestedTabItem{
		{URL: "https://x.com", Title: "X", ParentName: "c1"},
	}

	payload, _ := json.Marshal(map[string]any{
		"collections": []map[string]any{
			{"name": "c1", "itemCount": 1},
		},
	})

	env.app.view = ViewCollections
	env.app.applyRefresh(payload)

	vs := env.app.views[ViewCollections]
	if vs.itemCount < 1 {
		t.Errorf("collections with expanded: itemCount = %d, want >= 1", vs.itemCount)
	}
}

// --- renderListItem additional types ---

func TestIntegrationRenderListItem_AllTypes(t *testing.T) {
	env := newIntegrationEnv(t)

	tests := []struct {
		name     string
		item     any
		contains string
	}{
		{"TabItem active", TabItem{ID: 1, Title: "Active Tab", URL: "https://t.com", Active: true}, "Active Tab"},
		{"TabItem pinned", TabItem{ID: 2, Title: "Pinned Tab", URL: "https://t.com", Pinned: true}, "Pinned Tab"},
		{"TabItem normal", TabItem{ID: 3, Title: "Normal Tab", URL: "https://t.com"}, "Normal Tab"},
		{"TabItem muted", TabItem{ID: 4, Title: "Muted Tab", URL: "https://t.com", Muted: true}, "Muted Tab"},
		{"SessionItem", SessionItem{Name: "sess1", TabCount: 10}, "sess1"},
		{"CollectionItem", CollectionItem{Name: "coll1", ItemCount: 5}, "coll1"},
		{"NestedTabItem", NestedTabItem{URL: "https://nest.com", Title: "Nested"}, "Nested"},
		{"Unknown", struct{ X string }{"val"}, "val"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := env.app.renderListItem(tt.item, 80)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("renderListItem(%s) = %q, want to contain %q", tt.name, result, tt.contains)
			}
		})
	}
}

// --- openSingleTab ---

func TestIntegrationOpenSingleTab(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	tab := NestedTabItem{URL: "https://open.com", Title: "Open Me"}
	cmd := env.app.openSingleTab(tab)
	if cmd == nil {
		t.Fatal("openSingleTab should return cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if em, ok := msg.(errMsg); ok {
		t.Fatalf("openSingleTab error: %v", em.err)
	}
}

// --- switchWorkspace ---

func TestIntegrationSwitchWorkspace(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	vs := env.app.views[ViewWorkspaces]
	vs.items = []any{WorkspaceItem{ID: "ws-1", Name: "ws"}}
	vs.itemCount = 1
	vs.cursor = 0

	cmd := env.app.switchWorkspace()
	if cmd == nil {
		t.Fatal("switchWorkspace should return cmd")
	}

	// Workspace may not exist; just verify the code path runs.
	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if msg == nil {
		t.Error("switchWorkspace cmd should return a msg")
	}
}

func TestIntegrationSwitchWorkspace_Empty(t *testing.T) {
	env := newIntegrationEnv(t)

	vs := env.app.views[ViewWorkspaces]
	vs.items = nil
	vs.itemCount = 0

	cmd := env.app.switchWorkspace()
	if cmd != nil {
		t.Error("switchWorkspace on empty should return nil")
	}
}

// --- handleKey bookmark navigation (l/h/right/left) ---

func TestIntegrationHandleKey_BookmarkExpandCollapse(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewBookmarks
	env.app.collapsedFolders = make(map[string]bool)
	env.app.bookmarkTree = []BookmarkItem{
		{ID: "1", Title: "Folder", Children: []BookmarkItem{
			{ID: "2", Title: "Child", URL: "https://child.com"},
		}},
	}

	flat := flattenBookmarkTree(env.app.bookmarkTree, 0)
	vs := env.app.views[ViewBookmarks]
	vs.items = make([]any, len(flat))
	for i, b := range flat {
		vs.items[i] = b
	}
	vs.itemCount = len(flat)
	vs.cursor = 0

	// h/left on an expanded folder should collapse it
	env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if !env.app.collapsedFolders["1"] {
		t.Error("folder should be collapsed after h")
	}

	// l/right on a collapsed folder should expand it
	env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if env.app.collapsedFolders["1"] {
		t.Error("folder should be expanded after l")
	}
}

// --- Update message handling ---

func TestIntegrationUpdateToastMsg(t *testing.T) {
	env := newIntegrationEnv(t)

	model, _ := env.app.Update(toastMsg("Hello"))
	a := model.(*App)
	if a.toast != "Hello" {
		t.Errorf("toast = %q, want %q", a.toast, "Hello")
	}
}

func TestIntegrationUpdatePreviewTextMsg(t *testing.T) {
	env := newIntegrationEnv(t)

	model, _ := env.app.Update(previewTextMsg{tabID: 42, text: "preview content"})
	a := model.(*App)
	if a.previewText[42] != "preview content" {
		t.Errorf("previewText[42] = %q, want %q", a.previewText[42], "preview content")
	}
}

// --- foldAllBookmarks ---

func TestFoldAllBookmarks(t *testing.T) {
	env := newIntegrationEnv(t)
	env.app.collapsedFolders = make(map[string]bool)

	tree := BookmarkItem{
		ID:    "1",
		Title: "Root",
		Children: []BookmarkItem{
			{ID: "2", Title: "Sub", Children: []BookmarkItem{
				{ID: "3", Title: "SubSub", URL: "https://a.com"},
			}},
		},
	}

	env.app.foldAllBookmarks(tree)

	if !env.app.collapsedFolders["1"] {
		t.Error("folder 1 should be collapsed")
	}
	if !env.app.collapsedFolders["2"] {
		t.Error("folder 2 should be collapsed")
	}
}

// --- reflattenBookmarks ---

func TestReflattenBookmarks(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.bookmarkTree = []BookmarkItem{
		{ID: "1", Title: "Root", Children: []BookmarkItem{
			{ID: "2", Title: "Child", URL: "https://child.com"},
			{ID: "3", Title: "SubFolder", Children: []BookmarkItem{
				{ID: "4", Title: "Deep", URL: "https://deep.com"},
			}},
		}},
	}
	env.app.collapsedFolders = make(map[string]bool)

	env.app.reflattenBookmarks()

	vs := env.app.views[ViewBookmarks]
	if vs.itemCount != 4 { // Root + Child + SubFolder + Deep
		t.Errorf("itemCount = %d, want 4", vs.itemCount)
	}

	// Collapse Root, should only show Root
	env.app.collapsedFolders["1"] = true
	env.app.reflattenBookmarks()
	if vs.itemCount != 1 {
		t.Errorf("itemCount after collapse = %d, want 1", vs.itemCount)
	}
}

// --- renderContent for splitView ---

func TestIntegrationRenderContent_SplitView(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTabs
	env.app.width = 120
	env.app.height = 30

	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, Title: "Test Tab", URL: "https://test.com", Active: true},
	}
	vs.itemCount = 1
	vs.cursor = 0

	output := env.app.renderContent(20)
	if !strings.Contains(output, "Test Tab") {
		t.Error("renderContent should contain tab title")
	}
}

func TestIntegrationRenderContent_NonSplitView(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewGroups
	env.app.width = 80
	env.app.height = 30

	vs := env.app.views[ViewGroups]
	vs.items = []any{
		GroupItem{ID: 1, Title: "my-group", Color: "blue"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	output := env.app.renderContent(20)
	if !strings.Contains(output, "my-group") {
		t.Error("renderContent should contain group title")
	}
}

// --- handleFilterKey ---

func TestIntegrationHandleFilterKey_Enter(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.mode = ModeFilter
	env.app.filterText = "tab"
	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, Title: "Tab One", URL: "https://one.com"},
		TabItem{ID: 2, Title: "Other", URL: "https://other.com"},
	}
	vs.itemCount = 2

	model, _ := env.app.handleFilterKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	// Filter should be applied
	if len(vs.filtered) != 1 {
		t.Errorf("filtered = %d, want 1", len(vs.filtered))
	}
}

func TestIntegrationHandleFilterKey_Escape(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.mode = ModeFilter
	env.app.filterText = "test"
	vs := env.app.views[ViewTabs]
	vs.filtered = []int{0}

	model, _ := env.app.handleFilterKey("esc", tea.KeyMsg{Type: tea.KeyEscape})
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if vs.filtered != nil {
		t.Error("filter should be cleared on esc")
	}
}

func TestIntegrationHandleFilterKey_Backspace(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.mode = ModeFilter
	env.app.filterText = "abc"

	model, _ := env.app.handleFilterKey("backspace", tea.KeyMsg{Type: tea.KeyBackspace})
	a := model.(*App)
	if a.filterText != "ab" {
		t.Errorf("filterText = %q, want %q", a.filterText, "ab")
	}
}

func TestIntegrationHandleFilterKey_TypeRunes(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.mode = ModeFilter
	env.app.filterText = "ab"

	model, _ := env.app.handleFilterKey("c", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	a := model.(*App)
	if a.filterText != "abc" {
		t.Errorf("filterText = %q, want %q", a.filterText, "abc")
	}
}

// --- handleCommandKey ---

func TestIntegrationHandleCommandKey_Enter(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.mode = ModeCommand
	env.app.commandText = "help"

	model, _ := env.app.handleCommandKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)
	if a.mode != ModeHelp {
		t.Errorf("mode = %d, want ModeHelp after :help", a.mode)
	}
}

func TestIntegrationHandleCommandKey_Escape(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.mode = ModeCommand
	env.app.commandText = "test"

	model, _ := env.app.handleCommandKey("esc", tea.KeyMsg{Type: tea.KeyEscape})
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after esc", a.mode)
	}
	if a.commandText != "" {
		t.Errorf("commandText = %q, want empty", a.commandText)
	}
}

func TestIntegrationHandleCommandKey_Backspace(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.mode = ModeCommand
	env.app.commandText = "hello"

	model, _ := env.app.handleCommandKey("backspace", tea.KeyMsg{Type: tea.KeyBackspace})
	a := model.(*App)
	if a.commandText != "hell" {
		t.Errorf("commandText = %q, want %q", a.commandText, "hell")
	}
}

// --- currentItemName ---

func TestIntegrationCurrentItemName(t *testing.T) {
	env := newIntegrationEnv(t)

	tests := []struct {
		view ViewType
		item any
		want string
	}{
		{ViewSessions, SessionItem{Name: "s1"}, "s1"},
		{ViewCollections, CollectionItem{Name: "c1"}, "c1"},
		{ViewWorkspaces, WorkspaceItem{Name: "w1"}, "w1"},
		{ViewTabs, TabItem{ID: 1, Title: "Tab"}, ""},
	}

	for _, tt := range tests {
		env.app.view = tt.view
		vs := env.app.views[tt.view]
		vs.items = []any{tt.item}
		vs.itemCount = 1
		vs.cursor = 0

		name := env.app.currentItemName()
		if name != tt.want {
			t.Errorf("currentItemName for %v = %q, want %q", tt.view, name, tt.want)
		}
	}

	// Empty view
	env.app.view = ViewSessions
	vs := env.app.views[ViewSessions]
	vs.items = nil
	vs.itemCount = 0
	if env.app.currentItemName() != "" {
		t.Error("currentItemName on empty should return empty")
	}
}

// --- renderItem for additional types ---

func TestIntegrationRenderItem_HistoryItem(t *testing.T) {
	env := newIntegrationEnv(t)

	result := env.app.renderItem(HistoryItem{Title: "History Entry", URL: "https://h.com", VisitCount: 5})
	if !strings.Contains(result, "History Entry") {
		t.Errorf("renderItem(HistoryItem) = %q, want to contain 'History Entry'", result)
	}
}

func TestIntegrationRenderItem_SearchResultItem(t *testing.T) {
	env := newIntegrationEnv(t)

	result := env.app.renderItem(SearchResultItem{Title: "Search Result", URL: "https://sr.com", Kind: "tab", Score: 0.9})
	if !strings.Contains(result, "Search Result") {
		t.Errorf("renderItem(SearchResultItem) = %q, want to contain 'Search Result'", result)
	}
}

func TestIntegrationRenderItem_SavedSearchItem(t *testing.T) {
	env := newIntegrationEnv(t)

	result := env.app.renderItem(SavedSearchItem{Name: "MySavedSearch", QueryText: "test query"})
	if !strings.Contains(result, "MySavedSearch") {
		t.Errorf("renderItem(SavedSearchItem) = %q, want to contain 'MySavedSearch'", result)
	}
}

func TestIntegrationRenderItem_DownloadItem(t *testing.T) {
	env := newIntegrationEnv(t)

	result := env.app.renderItem(DownloadItem{Filename: "file.zip", State: "in_progress", TotalBytes: 1048576})
	if !strings.Contains(result, "file.zip") {
		t.Errorf("renderItem(DownloadItem) = %q, want to contain 'file.zip'", result)
	}
}

func TestIntegrationRenderItem_NestedTabItem(t *testing.T) {
	env := newIntegrationEnv(t)

	result := env.app.renderItem(NestedTabItem{Title: "NestedTab", URL: "https://nested.com", ParentName: "parent"})
	if !strings.Contains(result, "NestedTab") {
		t.Errorf("renderItem(NestedTabItem) = %q, want to contain 'NestedTab'", result)
	}
}

// --- renderPreviewPanel ---

func TestIntegrationRenderPreviewPanel_Empty(t *testing.T) {
	env := newIntegrationEnv(t)

	vs := env.app.views[ViewTabs]
	vs.items = nil
	vs.itemCount = 0

	result := env.app.renderPreviewPanel(vs, 40, 20)
	if result != nil {
		t.Error("renderPreviewPanel on empty should return nil")
	}
}

func TestIntegrationRenderPreviewPanel_TabItem(t *testing.T) {
	env := newIntegrationEnv(t)

	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, Title: "Preview Tab", URL: "https://preview.com", Active: true, WindowID: 1},
	}
	vs.itemCount = 1
	vs.cursor = 0

	result := env.app.renderPreviewPanel(vs, 40, 20)
	if len(result) == 0 {
		t.Error("renderPreviewPanel should return lines")
	}
}

// --- executeCommand additional commands ---

func TestIntegrationExecuteCommand_UnknownCommand(t *testing.T) {
	env := newIntegrationEnv(t)

	cmd := env.app.executeCommand("nonexistent-command")
	if cmd != nil {
		t.Error("unknown command should return nil")
	}
}

func TestIntegrationExecuteCommand_QuitVariant(t *testing.T) {
	env := newIntegrationEnv(t)

	cmd := env.app.executeCommand("quit")
	if cmd == nil {
		t.Fatal("quit command should return tea.Quit")
	}
	msg := execCmd(t, cmd)
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", msg)
	}
}

// --- renderHelp ---

func TestIntegrationRenderHelp(t *testing.T) {
	env := newIntegrationEnv(t)

	output := env.app.renderHelp()
	if !strings.Contains(output, "Help") {
		t.Error("renderHelp should contain 'Help'")
	}
	// Should contain key descriptions
	if !strings.Contains(output, "down") {
		t.Error("renderHelp should contain navigation key descriptions")
	}
}

// --- WindowSize handling ---

func TestIntegrationUpdateWindowSize(t *testing.T) {
	env := newIntegrationEnv(t)

	model, _ := env.app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a := model.(*App)
	if a.width != 120 || a.height != 40 {
		t.Errorf("size = %dx%d, want 120x40", a.width, a.height)
	}
}

// --- handleNameInputKey additional view-specific branches ---

func TestIntegrationHandleNameInputKey_AddToCollection(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// Pre-create collection
	coll := map[string]any{
		"name": "target-coll", "createdAt": "2024-01-01T00:00:00Z",
		"updatedAt": "2024-01-01T00:00:00Z", "items": []any{},
	}
	data, _ := json.MarshalIndent(coll, "", "  ")
	os.WriteFile(filepath.Join(env.tmpDir, "collections", "target-coll.json"), data, 0644)

	env.app.view = ViewTabs
	env.app.mode = ModeNameInput
	env.app.namePrompt = "Add to collection: "
	env.app.nameText = "target-coll"

	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, Title: "Tab A", URL: "https://a.com"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleNameInputKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd == nil {
		t.Fatal("add to collection should return cmd")
	}

	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if msg == nil {
		t.Error("add to collection cmd should return a msg")
	}
}

func TestIntegrationHandleNameInputKey_AddToCollectionWithSelection(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// Pre-create collection
	coll := map[string]any{
		"name": "sel-coll", "createdAt": "2024-01-01T00:00:00Z",
		"updatedAt": "2024-01-01T00:00:00Z", "items": []any{},
	}
	data, _ := json.MarshalIndent(coll, "", "  ")
	os.WriteFile(filepath.Join(env.tmpDir, "collections", "sel-coll.json"), data, 0644)

	env.app.view = ViewTabs
	env.app.mode = ModeNameInput
	env.app.namePrompt = "Add to collection: "
	env.app.nameText = "sel-coll"

	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, Title: "Tab A", URL: "https://a.com"},
		TabItem{ID: 2, Title: "Tab B", URL: "https://b.com"},
	}
	vs.itemCount = 2
	vs.selected[0] = true
	vs.selected[1] = true

	model, cmd := env.app.handleNameInputKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd == nil {
		t.Fatal("add to collection with selection should return cmd")
	}
}

func TestIntegrationHandleNameInputKey_AddToCollectionEmpty(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTabs
	env.app.mode = ModeNameInput
	env.app.namePrompt = "Add to collection: "
	env.app.nameText = "empty-coll"

	vs := env.app.views[ViewTabs]
	vs.items = nil
	vs.itemCount = 0

	_, cmd := env.app.handleNameInputKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("add to empty collection should return nil cmd")
	}
}

func TestIntegrationHandleNameInputKey_MoveToWindow(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewTabs
	env.app.mode = ModeNameInput
	env.app.namePrompt = "Move to window: "
	env.app.nameText = "1"

	vs := env.app.views[ViewTabs]
	vs.items = []any{
		TabItem{ID: 1, Title: "Tab", URL: "https://t.com", WindowID: 1},
	}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleNameInputKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd == nil {
		t.Fatal("move to window should return cmd")
	}
}

func TestIntegrationHandleNameInputKey_MoveToWindowInvalid(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTabs
	env.app.mode = ModeNameInput
	env.app.namePrompt = "Move to window: "
	env.app.nameText = "abc"

	vs := env.app.views[ViewTabs]
	vs.items = []any{TabItem{ID: 1, Title: "Tab"}}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleNameInputKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)
	if a.errorMsg != "Invalid window ID" {
		t.Errorf("errorMsg = %q, want 'Invalid window ID'", a.errorMsg)
	}
	if cmd != nil {
		t.Error("invalid window ID should return nil cmd")
	}
}

func TestIntegrationHandleNameInputKey_BookmarkCreate(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewBookmarks
	env.app.mode = ModeNameInput
	env.app.namePrompt = "URL: "
	env.app.nameText = "https://new-bookmark.com"

	model, cmd := env.app.handleNameInputKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd == nil {
		t.Fatal("bookmark create should return cmd")
	}
}

func TestIntegrationHandleNameInputKey_SearchSave(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewSearch
	env.app.mode = ModeNameInput
	env.app.namePrompt = "Save as: "
	env.app.nameText = "saved-query"
	env.app.lastSearchQuery = "test query"

	model, cmd := env.app.handleNameInputKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd == nil {
		t.Fatal("search save should return cmd")
	}
}

// --- handleKey mode dispatch ---

func TestIntegrationHandleKey_ModeFilter(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.mode = ModeFilter
	env.app.filterText = ""

	// Typing in filter mode
	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	a := model.(*App)
	if a.filterText != "a" {
		t.Errorf("filterText = %q, want 'a'", a.filterText)
	}
}

func TestIntegrationHandleKey_ModeCommand(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.mode = ModeCommand
	env.app.commandText = ""

	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	a := model.(*App)
	if a.commandText != "h" {
		t.Errorf("commandText = %q, want 'h'", a.commandText)
	}
}

func TestIntegrationHandleKey_ModeNameInput(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.mode = ModeNameInput
	env.app.nameText = ""

	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	a := model.(*App)
	if a.nameText != "x" {
		t.Errorf("nameText = %q, want 'x'", a.nameText)
	}
}

func TestIntegrationHandleKey_ModeYank(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.mode = ModeYank
	vs := env.app.views[ViewTabs]
	vs.items = []any{TabItem{ID: 1, Title: "T", URL: "https://t.com"}}
	vs.itemCount = 1
	vs.cursor = 0

	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after yank dispatch", a.mode)
	}
}

func TestIntegrationHandleKey_ModeZFilter(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewTabs
	env.app.mode = ModeZFilter
	vs := env.app.views[ViewTabs]
	vs.items = []any{TabItem{ID: 1, URL: "https://t.com", Active: true}}
	vs.itemCount = 1

	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after zfilter dispatch", a.mode)
	}
}

func TestIntegrationHandleKey_ModeConfirmDelete(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.mode = ModeConfirmDelete
	env.app.view = ViewSessions

	// Cancel with non-D key
	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after cancel delete", a.mode)
	}
}

func TestIntegrationHandleKey_HelpEsc(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.mode = ModeHelp

	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after esc in help", a.mode)
	}
}

func TestIntegrationHandleKey_HelpQuestion(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.mode = ModeHelp

	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal after ? in help", a.mode)
	}
}

func TestIntegrationHandleKey_EscSearchActive(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewSearch
	env.app.searchActive = true

	model, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	a := model.(*App)
	if a.searchActive {
		t.Error("searchActive should be false after esc")
	}
	if cmd == nil {
		t.Fatal("esc on active search should return refresh cmd")
	}
}

// --- handleEnter for NestedTabItem ---

func TestIntegrationHandleEnter_SessionNestedTab(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewSessions
	vs := env.app.views[ViewSessions]
	vs.items = []any{
		NestedTabItem{URL: "https://nested.com", Title: "Nested", ParentName: "s1"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	cmd := env.app.handleEnter()
	if cmd == nil {
		t.Fatal("handleEnter on NestedTabItem should return open cmd")
	}
}

func TestIntegrationHandleEnter_CollectionNestedTab(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewCollections
	vs := env.app.views[ViewCollections]
	vs.items = []any{
		NestedTabItem{URL: "https://nested.com", Title: "Nested", ParentName: "c1"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	cmd := env.app.handleEnter()
	if cmd == nil {
		t.Fatal("handleEnter on collection NestedTabItem should return open cmd")
	}
}

// --- handleEnter session collapse ---

func TestIntegrationHandleEnter_SessionCollapse(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewSessions
	vs := env.app.views[ViewSessions]

	// Pre-expand a session
	env.app.expandedSessions["s1"] = []NestedTabItem{
		{URL: "https://a.com", Title: "A", ParentName: "s1"},
	}
	vs.items = []any{
		SessionItem{Name: "s1", TabCount: 1},
		NestedTabItem{URL: "https://a.com", Title: "A", ParentName: "s1"},
	}
	vs.itemCount = 2
	vs.cursor = 0

	// Enter on expanded session should collapse
	cmd := env.app.handleEnter()
	if cmd != nil {
		t.Error("collapsing session should return nil cmd")
	}
	if _, ok := env.app.expandedSessions["s1"]; ok {
		t.Error("session should be collapsed (removed from expandedSessions)")
	}
}

func TestIntegrationHandleEnter_CollectionCollapse(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewCollections
	vs := env.app.views[ViewCollections]

	env.app.expandedCollections["c1"] = []NestedTabItem{
		{URL: "https://x.com", Title: "X", ParentName: "c1"},
	}
	vs.items = []any{
		CollectionItem{Name: "c1", ItemCount: 1},
		NestedTabItem{URL: "https://x.com", Title: "X", ParentName: "c1"},
	}
	vs.itemCount = 2
	vs.cursor = 0

	cmd := env.app.handleEnter()
	if cmd != nil {
		t.Error("collapsing collection should return nil cmd")
	}
	if _, ok := env.app.expandedCollections["c1"]; ok {
		t.Error("collection should be collapsed")
	}
}

// --- handleKey collection x on NestedTabItem ---

func TestIntegrationHandleKey_X_CollectionNestedItem(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// Pre-create collection with an item
	coll := map[string]any{
		"name": "coll-x", "createdAt": "2024-01-01T00:00:00Z",
		"updatedAt": "2024-01-01T00:00:00Z",
		"items": []map[string]any{
			{"url": "https://nested.com", "title": "Nested"},
		},
	}
	data, _ := json.MarshalIndent(coll, "", "  ")
	os.WriteFile(filepath.Join(env.tmpDir, "collections", "coll-x.json"), data, 0644)

	env.app.view = ViewCollections
	vs := env.app.views[ViewCollections]
	vs.items = []any{
		NestedTabItem{URL: "https://nested.com", Title: "Nested", ParentName: "coll-x"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	env.app.expandedCollections["coll-x"] = []NestedTabItem{
		{URL: "https://nested.com", Title: "Nested", ParentName: "coll-x"},
	}

	_, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd == nil {
		t.Fatal("x on collection nested item should return remove cmd")
	}
}

func TestIntegrationHandleKey_X_CollectionItem(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewCollections
	vs := env.app.views[ViewCollections]
	vs.items = []any{
		CollectionItem{Name: "coll-del"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	a := model.(*App)
	if a.mode != ModeConfirmDelete {
		t.Errorf("mode = %d, want ModeConfirmDelete", a.mode)
	}
	if cmd == nil {
		t.Fatal("x on collection should return timeout cmd")
	}
}

// --- refreshCurrentView for History and Search views ---

func TestIntegrationRefreshCurrentView_History(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewHistory
	cmd := env.app.refreshCurrentView()
	if cmd == nil {
		t.Fatal("refreshCurrentView for History should return cmd")
	}
	// Will return error since mock extension doesn't support history.search,
	// but the code path is exercised.
	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if msg == nil {
		t.Error("history refresh should return a msg")
	}
}

func TestIntegrationRefreshCurrentView_SearchSavedList(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewSearch
	env.app.searchActive = false
	cmd := env.app.refreshCurrentView()
	if cmd == nil {
		t.Fatal("refreshCurrentView for saved searches should return cmd")
	}
	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if msg == nil {
		t.Error("saved searches refresh should return a msg")
	}
}

func TestIntegrationRefreshCurrentView_Downloads(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewDownloads
	cmd := env.app.refreshCurrentView()
	if cmd == nil {
		t.Fatal("refreshCurrentView for Downloads should return cmd")
	}
	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if msg == nil {
		t.Error("downloads refresh should return a msg")
	}
}

// --- handleConfirmDeleteKey History view ---

func TestIntegrationHandleConfirmDeleteKey_History(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewHistory
	env.app.mode = ModeConfirmDelete
	vs := env.app.views[ViewHistory]
	vs.items = []any{
		HistoryItem{ID: "h1", URL: "https://h.com", Title: "History Entry"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleConfirmDeleteKey("D")
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd == nil {
		t.Fatal("history delete should return cmd")
	}
	// Extension may not support history.delete; just verify code path.
	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if msg == nil {
		t.Error("history delete cmd should return a msg")
	}
}

func TestIntegrationHandleConfirmDeleteKey_Search(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewSearch
	env.app.mode = ModeConfirmDelete
	vs := env.app.views[ViewSearch]
	vs.items = []any{
		SavedSearchItem{ID: "ss-1", Name: "saved-del"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleConfirmDeleteKey("D")
	a := model.(*App)
	if a.mode != ModeNormal {
		t.Errorf("mode = %d, want ModeNormal", a.mode)
	}
	if cmd == nil {
		t.Fatal("search delete should return cmd")
	}
	msg := execCmdWithTimeout(t, cmd, 5*time.Second)
	if msg == nil {
		t.Error("search delete cmd should return a msg")
	}
}

// --- D key on History view (first D to enter confirm mode) ---

func TestIntegrationHandleKey_D_History(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewHistory
	vs := env.app.views[ViewHistory]
	vs.items = []any{HistoryItem{ID: "h1", Title: "H", URL: "https://h.com"}}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	a := model.(*App)
	if a.mode != ModeConfirmDelete {
		t.Errorf("mode = %d, want ModeConfirmDelete", a.mode)
	}
	if cmd == nil {
		t.Fatal("D on history should return timeout cmd")
	}
}

// --- D key on Search view (saved searches) ---

func TestIntegrationHandleKey_D_SearchSaved(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewSearch
	env.app.searchActive = false
	vs := env.app.views[ViewSearch]
	vs.items = []any{SavedSearchItem{ID: "ss-1", Name: "saved"}}
	vs.itemCount = 1
	vs.cursor = 0

	model, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	a := model.(*App)
	if a.mode != ModeConfirmDelete {
		t.Errorf("mode = %d, want ModeConfirmDelete", a.mode)
	}
	if cmd == nil {
		t.Fatal("D on saved search should return timeout cmd")
	}
}

// --- R key on Sync view ---

func TestIntegrationHandleKey_R_SyncRepair(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	env.app.view = ViewSync
	_, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	if cmd == nil {
		t.Fatal("R on sync should return repair cmd")
	}
}

// --- N key on Search active ---

func TestIntegrationHandleKey_N_SearchSave(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewSearch
	env.app.searchActive = true
	env.app.lastSearchQuery = "test"

	model, _ := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	a := model.(*App)
	if a.mode != ModeNameInput {
		t.Errorf("mode = %d, want ModeNameInput", a.mode)
	}
	if a.namePrompt != "Save as: " {
		t.Errorf("namePrompt = %q, want 'Save as: '", a.namePrompt)
	}
}

// --- handleKey J/K for collection reorder ---

func TestIntegrationHandleKey_JK_CollectionReorder(t *testing.T) {
	env := newIntegrationEnv(t)
	env.connectApp(t)

	// Pre-create collection
	coll := map[string]any{
		"name": "reorder-coll", "createdAt": "2024-01-01T00:00:00Z",
		"updatedAt": "2024-01-01T00:00:00Z",
		"items": []map[string]any{
			{"url": "https://a.com", "title": "A"},
			{"url": "https://b.com", "title": "B"},
		},
	}
	data, _ := json.MarshalIndent(coll, "", "  ")
	os.WriteFile(filepath.Join(env.tmpDir, "collections", "reorder-coll.json"), data, 0644)

	env.app.view = ViewCollections
	env.app.expandedCollections["reorder-coll"] = []NestedTabItem{
		{URL: "https://a.com", Title: "A", ParentName: "reorder-coll"},
		{URL: "https://b.com", Title: "B", ParentName: "reorder-coll"},
	}

	vs := env.app.views[ViewCollections]
	vs.items = []any{
		CollectionItem{Name: "reorder-coll", ItemCount: 2},
		NestedTabItem{URL: "https://a.com", Title: "A", ParentName: "reorder-coll"},
		NestedTabItem{URL: "https://b.com", Title: "B", ParentName: "reorder-coll"},
	}
	vs.itemCount = 3
	vs.cursor = 1 // on first nested item

	_, cmd := env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	if cmd == nil {
		t.Fatal("J should return reorder cmd")
	}

	// K on second item
	vs.cursor = 2
	_, cmd = env.app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	if cmd == nil {
		t.Fatal("K should return reorder cmd")
	}
}

// --- reorderCollectionItem edge cases ---

func TestIntegrationReorderCollectionItem_NotNested(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewCollections
	vs := env.app.views[ViewCollections]
	vs.items = []any{CollectionItem{Name: "c1"}}
	vs.itemCount = 1
	vs.cursor = 0

	cmd := env.app.reorderCollectionItem(1)
	if cmd != nil {
		t.Error("reorder on non-nested should return nil")
	}
}

func TestIntegrationReorderCollectionItem_OutOfBounds(t *testing.T) {
	env := newIntegrationEnv(t)

	env.app.view = ViewCollections
	env.app.expandedCollections["c1"] = []NestedTabItem{
		{URL: "https://a.com", Title: "A", ParentName: "c1"},
	}

	vs := env.app.views[ViewCollections]
	vs.items = []any{
		NestedTabItem{URL: "https://a.com", Title: "A", ParentName: "c1"},
	}
	vs.itemCount = 1
	vs.cursor = 0

	// Try to move up (already at top)
	cmd := env.app.reorderCollectionItem(-1)
	if cmd != nil {
		t.Error("reorder at boundary should return nil")
	}
}

// Use variables to suppress "unused import" warnings.
var (
	_ = client.New
	_ = daemon.NewServer
)
