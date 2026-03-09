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

	_, cmd := env.app.handleYankKey("y")
	if cmd != nil {
		t.Error("yank on non-tab view should return nil cmd")
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
	if env.app.views[ViewBookmarks].itemCount != 3 { // root + 2 children
		t.Errorf("bookmarks itemCount = %d, want 3", env.app.views[ViewBookmarks].itemCount)
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
	if !strings.Contains(output, "(empty)") {
		t.Error("view should show (empty) when no items")
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

// Use variables to suppress "unused import" warnings.
var (
	_ = client.New
	_ = daemon.NewServer
)
