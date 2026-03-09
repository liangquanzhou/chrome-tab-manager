package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ctm/internal/protocol"
)

// testEnv sets up a daemon with Unix socket for testing.
type testEnv struct {
	server   *Server
	sockPath string
	cancel   context.CancelFunc
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	// Use /tmp with short names to stay within macOS 104-byte Unix socket path limit.
	dir, err := os.MkdirTemp("/tmp", "ctm-d-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sockPath := filepath.Join(dir, "test.sock")
	lockPath := filepath.Join(dir, "test.lock")
	sessionsDir := filepath.Join(dir, "sessions")
	collectionsDir := filepath.Join(dir, "collections")
	bookmarksDir := filepath.Join(dir, "bookmarks")
	overlaysDir := filepath.Join(dir, "overlays")
	workspacesDir := filepath.Join(dir, "workspaces")
	savedSearchesDir := filepath.Join(dir, "searches")
	syncCloudDir := filepath.Join(dir, "cloud")
	searchIndexPath := filepath.Join(dir, "search_index.json")

	os.MkdirAll(sessionsDir, 0700)
	os.MkdirAll(collectionsDir, 0700)
	os.MkdirAll(bookmarksDir, 0700)
	os.MkdirAll(overlaysDir, 0700)
	os.MkdirAll(workspacesDir, 0700)
	os.MkdirAll(savedSearchesDir, 0700)

	srv := NewServer(sockPath, lockPath, sessionsDir, collectionsDir, bookmarksDir, overlaysDir, workspacesDir, savedSearchesDir, syncCloudDir, searchIndexPath)
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	// Wait for socket to be ready
	for i := 0; i < 50; i++ {
		if _, err := net.Dial("unix", sockPath); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Cleanup(func() {
		cancel()
		<-errCh
	})

	return &testEnv{server: srv, sockPath: sockPath, cancel: cancel}
}

func dial(t *testing.T, sockPath string) (net.Conn, *protocol.Reader, *protocol.Writer) {
	t.Helper()
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn, protocol.NewReader(conn), protocol.NewWriter(conn)
}

func sendHello(t *testing.T, w *protocol.Writer, r *protocol.Reader) string {
	t.Helper()
	hello := &protocol.Message{
		ID:              protocol.MakeID(),
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeHello,
		Payload: mustMarshal(map[string]any{
			"channel":      "stable",
			"extensionId":  "test-ext",
			"instanceId":   "test-inst",
			"userAgent":    "Chrome/130.0 Test",
			"capabilities": []string{"tabs", "groups"},
		}),
	}
	if err := w.Write(hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	resp, err := r.Read()
	if err != nil {
		t.Fatalf("read hello response: %v", err)
	}

	var payload map[string]string
	json.Unmarshal(resp.Payload, &payload)
	return payload["targetId"]
}

func request(t *testing.T, w *protocol.Writer, r *protocol.Reader, action string, payload any) *protocol.Message {
	t.Helper()
	var rawPayload json.RawMessage
	if payload != nil {
		rawPayload = mustMarshal(payload)
	}

	msg := &protocol.Message{
		ID:              protocol.MakeID(),
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeRequest,
		Action:          action,
		Payload:         rawPayload,
	}
	if err := w.Write(msg); err != nil {
		t.Fatalf("write request: %v", err)
	}

	resp, err := r.Read()
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return resp
}

func TestHubTargetRegistration(t *testing.T) {
	env := newTestEnv(t)

	// Connect as extension and send hello
	_, extR, extW := dial(t, env.sockPath)
	targetID := sendHello(t, extW, extR)

	if targetID == "" {
		t.Fatal("targetID is empty")
	}

	// Connect as client and list targets
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "targets.list", nil)

	var data struct {
		Targets []struct {
			TargetID string `json:"targetId"`
		} `json:"targets"`
	}
	json.Unmarshal(resp.Payload, &data)

	if len(data.Targets) != 1 {
		t.Fatalf("targets count = %d, want 1", len(data.Targets))
	}
	if data.Targets[0].TargetID != targetID {
		t.Errorf("targetId = %q, want %q", data.Targets[0].TargetID, targetID)
	}
}

func TestHubRequestRouting(t *testing.T) {
	env := newTestEnv(t)

	// Extension
	extConn, extR, extW := dial(t, env.sockPath)
	_ = extConn
	sendHello(t, extW, extR)

	// Client
	_, cliR, cliW := dial(t, env.sockPath)

	// Client sends tabs.list → should be forwarded to extension
	reqID := protocol.MakeID()
	cliW.Write(&protocol.Message{
		ID:              reqID,
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeRequest,
		Action:          "tabs.list",
	})

	// Extension receives forwarded request
	extReq, err := extR.Read()
	if err != nil {
		t.Fatalf("ext read: %v", err)
	}
	if extReq.Action != "tabs.list" {
		t.Errorf("ext got action %q, want tabs.list", extReq.Action)
	}

	// Extension sends response
	extW.Write(&protocol.Message{
		ID:              extReq.ID,
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeResponse,
		Payload:         mustMarshal(map[string]any{"tabs": []any{}}),
	})

	// Client receives response
	cliResp, err := cliR.Read()
	if err != nil {
		t.Fatalf("cli read: %v", err)
	}
	if cliResp.Type != protocol.TypeResponse {
		t.Errorf("response type = %q, want response", cliResp.Type)
	}
}

func TestHubTargetDisconnect(t *testing.T) {
	env := newTestEnv(t)

	extConn, extR, extW := dial(t, env.sockPath)
	sendHello(t, extW, extR)

	// Disconnect extension
	extConn.Close()
	time.Sleep(100 * time.Millisecond)

	// List targets → should be empty
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "targets.list", nil)

	var data struct {
		Targets []any `json:"targets"`
	}
	json.Unmarshal(resp.Payload, &data)

	if len(data.Targets) != 0 {
		t.Errorf("targets count = %d, want 0 after disconnect", len(data.Targets))
	}
}

func TestHubSessionCRUD(t *testing.T) {
	env := newTestEnv(t)

	// Extension that responds to tabs.list and groups.list
	_, extR, extW := dial(t, env.sockPath)
	sendHello(t, extW, extR)

	go func() {
		for {
			msg, err := extR.Read()
			if err != nil {
				return
			}
			if msg.Type == protocol.TypeRequest {
				var respPayload any
				switch msg.Action {
				case "tabs.list":
					respPayload = map[string]any{
						"tabs": []map[string]any{
							{"id": 1, "windowId": 1, "index": 0, "title": "Test", "url": "https://test.com", "active": true, "pinned": false, "groupId": -1},
						},
					}
				case "groups.list":
					respPayload = map[string]any{"groups": []any{}}
				}
				extW.Write(&protocol.Message{
					ID:              msg.ID,
					ProtocolVersion: protocol.ProtocolVersion,
					Type:            protocol.TypeResponse,
					Payload:         mustMarshal(respPayload),
				})
			}
		}
	}()

	_, cliR, cliW := dial(t, env.sockPath)

	// Save
	resp := request(t, cliW, cliR, "sessions.save", map[string]string{"name": "test-session"})
	if resp.Type == protocol.TypeError {
		t.Fatalf("save error: %s", resp.Error.Message)
	}

	var saveResult struct {
		Name     string `json:"name"`
		TabCount int    `json:"tabCount"`
	}
	json.Unmarshal(resp.Payload, &saveResult)
	if saveResult.Name != "test-session" {
		t.Errorf("name = %q, want test-session", saveResult.Name)
	}
	if saveResult.TabCount != 1 {
		t.Errorf("tabCount = %d, want 1", saveResult.TabCount)
	}

	// List (should return summary, not tabs)
	resp = request(t, cliW, cliR, "sessions.list", nil)
	var listResult struct {
		Sessions []struct {
			Name     string `json:"name"`
			TabCount int    `json:"tabCount"`
		} `json:"sessions"`
	}
	json.Unmarshal(resp.Payload, &listResult)
	if len(listResult.Sessions) != 1 {
		t.Fatalf("sessions count = %d, want 1", len(listResult.Sessions))
	}
	if listResult.Sessions[0].TabCount != 1 {
		t.Errorf("list tabCount = %d, want 1", listResult.Sessions[0].TabCount)
	}

	// Get (should return full data with tabs)
	resp = request(t, cliW, cliR, "sessions.get", map[string]string{"name": "test-session"})
	var getResult struct {
		Session struct {
			Windows []struct {
				Tabs []struct {
					URL string `json:"url"`
				} `json:"tabs"`
			} `json:"windows"`
		} `json:"session"`
	}
	json.Unmarshal(resp.Payload, &getResult)
	if len(getResult.Session.Windows) != 1 {
		t.Fatalf("windows = %d, want 1", len(getResult.Session.Windows))
	}
	if getResult.Session.Windows[0].Tabs[0].URL != "https://test.com" {
		t.Errorf("tab url = %q, want https://test.com", getResult.Session.Windows[0].Tabs[0].URL)
	}

	// Delete
	resp = request(t, cliW, cliR, "sessions.delete", map[string]string{"name": "test-session"})
	if resp.Type == protocol.TypeError {
		t.Fatalf("delete error: %s", resp.Error.Message)
	}

	// List → empty
	resp = request(t, cliW, cliR, "sessions.list", nil)
	json.Unmarshal(resp.Payload, &listResult)
	if len(listResult.Sessions) != 0 {
		t.Errorf("sessions count after delete = %d, want 0", len(listResult.Sessions))
	}
}

func TestHubCollectionCRUD(t *testing.T) {
	env := newTestEnv(t)

	_, cliR, cliW := dial(t, env.sockPath)

	// Create
	resp := request(t, cliW, cliR, "collections.create", map[string]string{"name": "favorites"})
	if resp.Type == protocol.TypeError {
		t.Fatalf("create error: %s", resp.Error.Message)
	}

	// AddItems
	resp = request(t, cliW, cliR, "collections.addItems", map[string]any{
		"name": "favorites",
		"items": []map[string]string{
			{"url": "https://example.com", "title": "Example"},
			{"url": "https://test.com", "title": "Test"},
		},
	})
	var addResult struct {
		ItemCount int `json:"itemCount"`
	}
	json.Unmarshal(resp.Payload, &addResult)
	if addResult.ItemCount != 2 {
		t.Errorf("itemCount after add = %d, want 2", addResult.ItemCount)
	}

	// Get (full data with items)
	resp = request(t, cliW, cliR, "collections.get", map[string]string{"name": "favorites"})
	var getResult struct {
		Collection struct {
			Items []struct {
				URL string `json:"url"`
			} `json:"items"`
		} `json:"collection"`
	}
	json.Unmarshal(resp.Payload, &getResult)
	if len(getResult.Collection.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(getResult.Collection.Items))
	}

	// RemoveItems
	resp = request(t, cliW, cliR, "collections.removeItems", map[string]any{
		"name": "favorites",
		"urls": []string{"https://example.com"},
	})
	var removeResult struct {
		ItemCount int `json:"itemCount"`
	}
	json.Unmarshal(resp.Payload, &removeResult)
	if removeResult.ItemCount != 1 {
		t.Errorf("itemCount after remove = %d, want 1", removeResult.ItemCount)
	}

	// Delete
	resp = request(t, cliW, cliR, "collections.delete", map[string]string{"name": "favorites"})
	if resp.Type == protocol.TypeError {
		t.Fatalf("delete error: %s", resp.Error.Message)
	}

	// List → empty
	resp = request(t, cliW, cliR, "collections.list", nil)
	var listResult struct {
		Collections []any `json:"collections"`
	}
	json.Unmarshal(resp.Payload, &listResult)
	if len(listResult.Collections) != 0 {
		t.Errorf("collections after delete = %d, want 0", len(listResult.Collections))
	}
}

func TestHubSubscribeAndFanout(t *testing.T) {
	env := newTestEnv(t)

	_, extR, extW := dial(t, env.sockPath)
	sendHello(t, extW, extR)

	// Subscribe as client
	_, subR, subW := dial(t, env.sockPath)
	resp := request(t, subW, subR, "subscribe", map[string]any{"patterns": []string{"tabs.*"}})
	if resp.Type == protocol.TypeError {
		t.Fatalf("subscribe error: %s", resp.Error.Message)
	}

	// Extension sends an event
	extW.Write(&protocol.Message{
		ID:              "evt_1",
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeEvent,
		Action:          "tabs.created",
		Payload:         mustMarshal(map[string]any{"tab": map[string]any{"id": 1}, "_target": map[string]string{"targetId": "target_1"}}),
	})

	// Subscriber should receive it
	event, err := subR.Read()
	if err != nil {
		t.Fatalf("read event: %v", err)
	}
	if event.Action != "tabs.created" {
		t.Errorf("event action = %q, want tabs.created", event.Action)
	}

	// Verify _target is present
	var payload map[string]json.RawMessage
	json.Unmarshal(event.Payload, &payload)
	if _, ok := payload["_target"]; !ok {
		t.Error("event payload missing _target")
	}
}

func TestHubEventPatternFiltering(t *testing.T) {
	env := newTestEnv(t)

	_, extR, extW := dial(t, env.sockPath)
	sendHello(t, extW, extR)

	// Subscribe only to tabs.*
	_, subR, subW := dial(t, env.sockPath)
	request(t, subW, subR, "subscribe", map[string]any{"patterns": []string{"tabs.*"}})

	// Send tabs event → should arrive
	extW.Write(&protocol.Message{
		ID: "evt_tabs", ProtocolVersion: protocol.ProtocolVersion,
		Type: protocol.TypeEvent, Action: "tabs.created",
		Payload: mustMarshal(map[string]any{"_target": map[string]string{"targetId": "t1"}}),
	})

	// Send groups event → should NOT arrive
	extW.Write(&protocol.Message{
		ID: "evt_groups", ProtocolVersion: protocol.ProtocolVersion,
		Type: protocol.TypeEvent, Action: "groups.created",
		Payload: mustMarshal(map[string]any{"_target": map[string]string{"targetId": "t1"}}),
	})

	// Read with timeout — should only get tabs event
	subConn := subR
	evt, err := subConn.Read()
	if err != nil {
		t.Fatalf("read event: %v", err)
	}
	if evt.Action != "tabs.created" {
		t.Errorf("got action %q, want tabs.created", evt.Action)
	}

	// Try reading again with short timeout — should get nothing
	done := make(chan *protocol.Message, 1)
	go func() {
		msg, _ := subConn.Read()
		done <- msg
	}()

	select {
	case msg := <-done:
		if msg != nil && msg.Action == "groups.created" {
			t.Error("received groups event despite tabs.* filter")
		}
	case <-time.After(200 * time.Millisecond):
		// Expected: no more events
	}
}

func TestHubFlockSingleton(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	f1, err := acquireLock(lockPath)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	defer f1.Close()

	_, err = acquireLock(lockPath)
	if err == nil {
		t.Error("second lock should fail")
	}
}

func TestHubNameValidationOnSave(t *testing.T) {
	env := newTestEnv(t)

	_, cliR, cliW := dial(t, env.sockPath)

	badNames := []string{"../evil", "", "has space", "has/slash"}
	for _, name := range badNames {
		resp := request(t, cliW, cliR, "sessions.save", map[string]string{"name": name})
		if resp.Type != protocol.TypeError {
			t.Errorf("name %q should be rejected", name)
		}
	}
}

func TestPatternMatching(t *testing.T) {
	tests := []struct {
		action  string
		pattern string
		want    bool
	}{
		{"tabs.created", "tabs.*", true},
		{"tabs.removed", "tabs.*", true},
		{"groups.created", "tabs.*", false},
		{"tabs.created", "*", true},
		{"anything", "*", true},
		{"tabs.created", "tabs.created", true},
		{"tabs.removed", "tabs.created", false},
	}
	for _, tt := range tests {
		got := matchPattern(tt.action, tt.pattern)
		if got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.action, tt.pattern, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// 1. Target handler tests
// ---------------------------------------------------------------------------

func TestHubTargetsDefault(t *testing.T) {
	env := newTestEnv(t)

	_, extR, extW := dial(t, env.sockPath)
	targetID := sendHello(t, extW, extR)

	_, cliR, cliW := dial(t, env.sockPath)

	// Set default to existing target → success
	resp := request(t, cliW, cliR, "targets.default", map[string]string{"targetId": targetID})
	if resp.Type == protocol.TypeError {
		t.Fatalf("targets.default error: %s", resp.Error.Message)
	}
	var defResult struct {
		TargetID string `json:"targetId"`
	}
	json.Unmarshal(resp.Payload, &defResult)
	if defResult.TargetID != targetID {
		t.Errorf("targetId = %q, want %q", defResult.TargetID, targetID)
	}

	// Set default to non-existent target → error with ErrTargetOffline
	resp = request(t, cliW, cliR, "targets.default", map[string]string{"targetId": "nonexistent_target"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for non-existent target")
	}
	if resp.Error.Code != protocol.ErrTargetOffline {
		t.Errorf("error code = %q, want %q", resp.Error.Code, protocol.ErrTargetOffline)
	}
}

func TestHubTargetsClearDefault(t *testing.T) {
	env := newTestEnv(t)

	// Register two targets
	_, ext1R, ext1W := dial(t, env.sockPath)
	target1ID := sendHello(t, ext1W, ext1R)
	// Drain ext1 messages in background
	go func() {
		for {
			if _, err := ext1R.Read(); err != nil {
				return
			}
		}
	}()

	_, ext2R, ext2W := dial(t, env.sockPath)
	_ = sendHello(t, ext2W, ext2R)
	go func() {
		for {
			if _, err := ext2R.Read(); err != nil {
				return
			}
		}
	}()

	_, cliR, cliW := dial(t, env.sockPath)

	// Set target1 as default
	resp := request(t, cliW, cliR, "targets.default", map[string]string{"targetId": target1ID})
	if resp.Type == protocol.TypeError {
		t.Fatalf("set default error: %s", resp.Error.Message)
	}

	// Clear default
	resp = request(t, cliW, cliR, "targets.clearDefault", nil)
	if resp.Type == protocol.TypeError {
		t.Fatalf("clearDefault error: %s", resp.Error.Message)
	}

	// Now with 2 targets and no default, tabs.list should return TARGET_AMBIGUOUS
	resp = request(t, cliW, cliR, "tabs.list", nil)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for ambiguous target after clearing default")
	}
	if resp.Error.Code != protocol.ErrTargetAmbiguous {
		t.Errorf("error code = %q, want %q", resp.Error.Code, protocol.ErrTargetAmbiguous)
	}
}

func TestHubTargetsLabel(t *testing.T) {
	env := newTestEnv(t)

	_, extR, extW := dial(t, env.sockPath)
	targetID := sendHello(t, extW, extR)

	_, cliR, cliW := dial(t, env.sockPath)

	// Set label → success
	resp := request(t, cliW, cliR, "targets.label", map[string]string{
		"targetId": targetID,
		"label":    "work",
	})
	if resp.Type == protocol.TypeError {
		t.Fatalf("targets.label error: %s", resp.Error.Message)
	}
	var labelResult struct {
		TargetID string `json:"targetId"`
		Label    string `json:"label"`
	}
	json.Unmarshal(resp.Payload, &labelResult)
	if labelResult.Label != "work" {
		t.Errorf("label = %q, want %q", labelResult.Label, "work")
	}

	// Set label on non-existent target → error
	resp = request(t, cliW, cliR, "targets.label", map[string]string{
		"targetId": "nonexistent_target",
		"label":    "test",
	})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for non-existent target")
	}
	if resp.Error.Code != protocol.ErrTargetOffline {
		t.Errorf("error code = %q, want %q", resp.Error.Code, protocol.ErrTargetOffline)
	}

	// Verify label shows in list
	resp = request(t, cliW, cliR, "targets.list", nil)
	var listResult struct {
		Targets []struct {
			TargetID string `json:"targetId"`
			Label    string `json:"label"`
		} `json:"targets"`
	}
	json.Unmarshal(resp.Payload, &listResult)
	if len(listResult.Targets) != 1 {
		t.Fatalf("targets count = %d, want 1", len(listResult.Targets))
	}
	if listResult.Targets[0].Label != "work" {
		t.Errorf("listed label = %q, want %q", listResult.Targets[0].Label, "work")
	}
}

// ---------------------------------------------------------------------------
// 2. resolveTarget coverage tests
// ---------------------------------------------------------------------------

func TestHubResolveExplicit(t *testing.T) {
	env := newTestEnv(t)

	_, extR, extW := dial(t, env.sockPath)
	targetID := sendHello(t, extW, extR)

	// Extension mock: respond to forwarded requests
	go func() {
		for {
			msg, err := extR.Read()
			if err != nil {
				return
			}
			if msg.Type == protocol.TypeRequest {
				extW.Write(&protocol.Message{
					ID:              msg.ID,
					ProtocolVersion: protocol.ProtocolVersion,
					Type:            protocol.TypeResponse,
					Payload:         mustMarshal(map[string]any{"tabs": []any{}}),
				})
			}
		}
	}()

	// Client sends tabs.list with explicit _target
	_, cliR, cliW := dial(t, env.sockPath)
	msg := &protocol.Message{
		ID:              protocol.MakeID(),
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeRequest,
		Action:          "tabs.list",
		Target:          &protocol.TargetSelector{TargetID: targetID},
	}
	if err := cliW.Write(msg); err != nil {
		t.Fatalf("write: %v", err)
	}
	resp, err := cliR.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if resp.Type == protocol.TypeError {
		t.Fatalf("expected success, got error: %s", resp.Error.Message)
	}
}

func TestHubResolveDefault(t *testing.T) {
	env := newTestEnv(t)

	// Register 2 targets
	_, ext1R, ext1W := dial(t, env.sockPath)
	target1ID := sendHello(t, ext1W, ext1R)
	go func() {
		for {
			msg, err := ext1R.Read()
			if err != nil {
				return
			}
			if msg.Type == protocol.TypeRequest {
				ext1W.Write(&protocol.Message{
					ID:              msg.ID,
					ProtocolVersion: protocol.ProtocolVersion,
					Type:            protocol.TypeResponse,
					Payload:         mustMarshal(map[string]any{"tabs": []any{}, "source": "target1"}),
				})
			}
		}
	}()

	_, ext2R, ext2W := dial(t, env.sockPath)
	_ = sendHello(t, ext2W, ext2R)
	go func() {
		for {
			msg, err := ext2R.Read()
			if err != nil {
				return
			}
			if msg.Type == protocol.TypeRequest {
				ext2W.Write(&protocol.Message{
					ID:              msg.ID,
					ProtocolVersion: protocol.ProtocolVersion,
					Type:            protocol.TypeResponse,
					Payload:         mustMarshal(map[string]any{"tabs": []any{}, "source": "target2"}),
				})
			}
		}
	}()

	_, cliR, cliW := dial(t, env.sockPath)

	// Set target1 as default
	resp := request(t, cliW, cliR, "targets.default", map[string]string{"targetId": target1ID})
	if resp.Type == protocol.TypeError {
		t.Fatalf("set default error: %s", resp.Error.Message)
	}

	// Send tabs.list without target → should route to default (target1)
	resp = request(t, cliW, cliR, "tabs.list", nil)
	if resp.Type == protocol.TypeError {
		t.Fatalf("tabs.list error: %s", resp.Error.Message)
	}
	var result struct {
		Source string `json:"source"`
	}
	json.Unmarshal(resp.Payload, &result)
	if result.Source != "target1" {
		t.Errorf("source = %q, want %q (should route to default)", result.Source, "target1")
	}
}

func TestHubResolveFallback(t *testing.T) {
	env := newTestEnv(t)

	// Register only 1 target, clear default to test fallback
	_, extR, extW := dial(t, env.sockPath)
	sendHello(t, extW, extR)
	go func() {
		for {
			msg, err := extR.Read()
			if err != nil {
				return
			}
			if msg.Type == protocol.TypeRequest {
				extW.Write(&protocol.Message{
					ID:              msg.ID,
					ProtocolVersion: protocol.ProtocolVersion,
					Type:            protocol.TypeResponse,
					Payload:         mustMarshal(map[string]any{"tabs": []any{}}),
				})
			}
		}
	}()

	_, cliR, cliW := dial(t, env.sockPath)

	// Clear default (even though auto-set, let's clear it)
	request(t, cliW, cliR, "targets.clearDefault", nil)

	// With 1 target and no default, single fallback should work
	resp := request(t, cliW, cliR, "tabs.list", nil)
	if resp.Type == protocol.TypeError {
		t.Fatalf("single fallback should work, got error: %s", resp.Error.Message)
	}
}

func TestHubResolveAmbiguous(t *testing.T) {
	env := newTestEnv(t)

	_, ext1R, ext1W := dial(t, env.sockPath)
	sendHello(t, ext1W, ext1R)
	go func() {
		for {
			if _, err := ext1R.Read(); err != nil {
				return
			}
		}
	}()

	_, ext2R, ext2W := dial(t, env.sockPath)
	sendHello(t, ext2W, ext2R)
	go func() {
		for {
			if _, err := ext2R.Read(); err != nil {
				return
			}
		}
	}()

	_, cliR, cliW := dial(t, env.sockPath)

	// Clear default
	request(t, cliW, cliR, "targets.clearDefault", nil)

	// 2 targets, no default → ambiguous
	resp := request(t, cliW, cliR, "tabs.list", nil)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for ambiguous targets")
	}
	if resp.Error.Code != protocol.ErrTargetAmbiguous {
		t.Errorf("error code = %q, want %q", resp.Error.Code, protocol.ErrTargetAmbiguous)
	}
}

func TestHubResolveNoTargets(t *testing.T) {
	env := newTestEnv(t)

	_, cliR, cliW := dial(t, env.sockPath)

	resp := request(t, cliW, cliR, "tabs.list", nil)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error when no targets connected")
	}
	if resp.Error.Code != protocol.ErrExtensionNotConnected {
		t.Errorf("error code = %q, want %q", resp.Error.Code, protocol.ErrExtensionNotConnected)
	}
}

// ---------------------------------------------------------------------------
// 3. targetErrorCode tests
// ---------------------------------------------------------------------------

func TestTargetErrorCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want protocol.ErrorCode
	}{
		{"target not found", &TargetNotFoundError{TargetID: "xyz"}, protocol.ErrTargetOffline},
		{"no targets connected", ErrNoTargetsConnected, protocol.ErrExtensionNotConnected},
		{"multiple targets ambiguous", ErrMultipleTargetsAmbig, protocol.ErrTargetAmbiguous},
		{"wrapped target not found", fmt.Errorf("resolve: %w", &TargetNotFoundError{TargetID: "abc"}), protocol.ErrTargetOffline},
		{"wrapped no targets", fmt.Errorf("resolve: %w", ErrNoTargetsConnected), protocol.ErrExtensionNotConnected},
		{"unknown error fallback", fmt.Errorf("something else entirely"), protocol.ErrTargetAmbiguous},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := targetErrorCode(tt.err)
			if got != tt.want {
				t.Errorf("targetErrorCode(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 4. Session restore tests
// ---------------------------------------------------------------------------

func TestHubSessionsRestore(t *testing.T) {
	env := newTestEnv(t)

	tabCounter := 0

	_, extR, extW := dial(t, env.sockPath)
	sendHello(t, extW, extR)

	go func() {
		for {
			msg, err := extR.Read()
			if err != nil {
				return
			}
			if msg.Type == protocol.TypeRequest {
				var respPayload any
				switch msg.Action {
				case "tabs.list":
					respPayload = map[string]any{
						"tabs": []map[string]any{
							{"id": 1, "windowId": 1, "index": 0, "title": "Tab A", "url": "https://a.com", "active": true, "pinned": false, "groupId": 100},
							{"id": 2, "windowId": 1, "index": 1, "title": "Tab B", "url": "https://b.com", "active": false, "pinned": false, "groupId": 100},
							{"id": 3, "windowId": 1, "index": 2, "title": "Tab C", "url": "https://c.com", "active": false, "pinned": false, "groupId": -1},
						},
					}
				case "groups.list":
					respPayload = map[string]any{
						"groups": []map[string]any{
							{"id": 100, "title": "Dev", "color": "blue", "collapsed": false},
						},
					}
				case "tabs.open":
					tabCounter++
					respPayload = map[string]any{"tabId": tabCounter}
				case "groups.create":
					respPayload = map[string]any{"groupId": 1}
				default:
					respPayload = map[string]any{}
				}
				extW.Write(&protocol.Message{
					ID:              msg.ID,
					ProtocolVersion: protocol.ProtocolVersion,
					Type:            protocol.TypeResponse,
					Payload:         mustMarshal(respPayload),
				})
			}
		}
	}()

	_, cliR, cliW := dial(t, env.sockPath)

	// Save session first
	resp := request(t, cliW, cliR, "sessions.save", map[string]string{"name": "restore-test"})
	if resp.Type == protocol.TypeError {
		t.Fatalf("save error: %s", resp.Error.Message)
	}

	// Restore session
	resp = request(t, cliW, cliR, "sessions.restore", map[string]string{"name": "restore-test"})
	if resp.Type == protocol.TypeError {
		t.Fatalf("restore error: %s", resp.Error.Message)
	}

	var restoreResult struct {
		WindowsCreated int `json:"windowsCreated"`
		TabsOpened     int `json:"tabsOpened"`
		TabsFailed     int `json:"tabsFailed"`
		GroupsCreated  int `json:"groupsCreated"`
	}
	json.Unmarshal(resp.Payload, &restoreResult)

	if restoreResult.WindowsCreated != 1 {
		t.Errorf("windowsCreated = %d, want 1", restoreResult.WindowsCreated)
	}
	if restoreResult.TabsOpened != 3 {
		t.Errorf("tabsOpened = %d, want 3", restoreResult.TabsOpened)
	}
	if restoreResult.TabsFailed != 0 {
		t.Errorf("tabsFailed = %d, want 0", restoreResult.TabsFailed)
	}
	if restoreResult.GroupsCreated != 1 {
		t.Errorf("groupsCreated = %d, want 1", restoreResult.GroupsCreated)
	}
}

func TestHubSessionsRestoreNotFound(t *testing.T) {
	env := newTestEnv(t)

	_, extR, extW := dial(t, env.sockPath)
	sendHello(t, extW, extR)
	go func() {
		for {
			if _, err := extR.Read(); err != nil {
				return
			}
		}
	}()

	_, cliR, cliW := dial(t, env.sockPath)

	resp := request(t, cliW, cliR, "sessions.restore", map[string]string{"name": "nonexistent-session"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for non-existent session restore")
	}
}

func TestHubSessionsRestoreNoTarget(t *testing.T) {
	env := newTestEnv(t)

	// Manually write a session file so restore finds it but no target is connected
	session := &Session{
		Name:      "orphan-session",
		CreatedAt: "2025-01-01T00:00:00Z",
		Windows: []SessionWindow{
			{Tabs: []SessionTab{{URL: "https://example.com", Title: "Example"}}},
		},
	}
	atomicWriteJSON(filepath.Join(filepath.Dir(env.sockPath), "sessions"), "orphan-session.json", session)

	_, cliR, cliW := dial(t, env.sockPath)

	resp := request(t, cliW, cliR, "sessions.restore", map[string]string{"name": "orphan-session"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error when no target connected for restore")
	}
	if resp.Error.Code != protocol.ErrExtensionNotConnected {
		t.Errorf("error code = %q, want %q", resp.Error.Code, protocol.ErrExtensionNotConnected)
	}
}

// ---------------------------------------------------------------------------
// 5. Collection restore tests
// ---------------------------------------------------------------------------

func TestHubCollectionsRestore(t *testing.T) {
	env := newTestEnv(t)

	tabCounter := 0
	_, extR, extW := dial(t, env.sockPath)
	sendHello(t, extW, extR)

	go func() {
		for {
			msg, err := extR.Read()
			if err != nil {
				return
			}
			if msg.Type == protocol.TypeRequest {
				var respPayload any
				switch msg.Action {
				case "tabs.open":
					tabCounter++
					respPayload = map[string]any{"tabId": tabCounter}
				default:
					respPayload = map[string]any{}
				}
				extW.Write(&protocol.Message{
					ID:              msg.ID,
					ProtocolVersion: protocol.ProtocolVersion,
					Type:            protocol.TypeResponse,
					Payload:         mustMarshal(respPayload),
				})
			}
		}
	}()

	_, cliR, cliW := dial(t, env.sockPath)

	// Create collection and add items
	resp := request(t, cliW, cliR, "collections.create", map[string]string{"name": "restore-col"})
	if resp.Type == protocol.TypeError {
		t.Fatalf("create collection error: %s", resp.Error.Message)
	}

	resp = request(t, cliW, cliR, "collections.addItems", map[string]any{
		"name": "restore-col",
		"items": []map[string]string{
			{"url": "https://one.com", "title": "One"},
			{"url": "https://two.com", "title": "Two"},
			{"url": "https://three.com", "title": "Three"},
		},
	})
	if resp.Type == protocol.TypeError {
		t.Fatalf("addItems error: %s", resp.Error.Message)
	}

	// Restore collection
	resp = request(t, cliW, cliR, "collections.restore", map[string]string{"name": "restore-col"})
	if resp.Type == protocol.TypeError {
		t.Fatalf("restore error: %s", resp.Error.Message)
	}

	var restoreResult struct {
		TabsOpened int `json:"tabsOpened"`
		TabsFailed int `json:"tabsFailed"`
	}
	json.Unmarshal(resp.Payload, &restoreResult)

	if restoreResult.TabsOpened != 3 {
		t.Errorf("tabsOpened = %d, want 3", restoreResult.TabsOpened)
	}
	if restoreResult.TabsFailed != 0 {
		t.Errorf("tabsFailed = %d, want 0", restoreResult.TabsFailed)
	}
}

func TestHubCollectionsRestoreNotFound(t *testing.T) {
	env := newTestEnv(t)

	_, extR, extW := dial(t, env.sockPath)
	sendHello(t, extW, extR)
	go func() {
		for {
			if _, err := extR.Read(); err != nil {
				return
			}
		}
	}()

	_, cliR, cliW := dial(t, env.sockPath)

	resp := request(t, cliW, cliR, "collections.restore", map[string]string{"name": "nonexistent-col"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for non-existent collection restore")
	}
}

func TestHubCollectionsRestoreEmpty(t *testing.T) {
	env := newTestEnv(t)

	_, extR, extW := dial(t, env.sockPath)
	sendHello(t, extW, extR)
	go func() {
		for {
			msg, err := extR.Read()
			if err != nil {
				return
			}
			if msg.Type == protocol.TypeRequest {
				extW.Write(&protocol.Message{
					ID:              msg.ID,
					ProtocolVersion: protocol.ProtocolVersion,
					Type:            protocol.TypeResponse,
					Payload:         mustMarshal(map[string]any{}),
				})
			}
		}
	}()

	_, cliR, cliW := dial(t, env.sockPath)

	// Create empty collection
	resp := request(t, cliW, cliR, "collections.create", map[string]string{"name": "empty-col"})
	if resp.Type == protocol.TypeError {
		t.Fatalf("create error: %s", resp.Error.Message)
	}

	// Restore empty collection
	resp = request(t, cliW, cliR, "collections.restore", map[string]string{"name": "empty-col"})
	if resp.Type == protocol.TypeError {
		t.Fatalf("restore error: %s", resp.Error.Message)
	}

	var restoreResult struct {
		TabsOpened int `json:"tabsOpened"`
		TabsFailed int `json:"tabsFailed"`
	}
	json.Unmarshal(resp.Payload, &restoreResult)

	if restoreResult.TabsOpened != 0 {
		t.Errorf("tabsOpened = %d, want 0", restoreResult.TabsOpened)
	}
	if restoreResult.TabsFailed != 0 {
		t.Errorf("tabsFailed = %d, want 0", restoreResult.TabsFailed)
	}
}

// ---------------------------------------------------------------------------
// 6. Bookmarks handler tests
// ---------------------------------------------------------------------------

func TestHubBookmarksOverlaySetGet(t *testing.T) {
	env := newTestEnv(t)

	_, cliR, cliW := dial(t, env.sockPath)

	// Set overlay
	resp := request(t, cliW, cliR, "bookmarks.overlay.set", map[string]any{
		"bookmarkId": "bm_123",
		"tags":       []string{"dev", "go"},
		"note":       "important bookmark",
		"alias":      "my-bookmark",
	})
	if resp.Type == protocol.TypeError {
		t.Fatalf("overlay.set error: %s", resp.Error.Message)
	}

	var setResult struct {
		BookmarkID string   `json:"bookmarkId"`
		Tags       []string `json:"tags"`
		Note       string   `json:"note"`
		Alias      string   `json:"alias"`
	}
	json.Unmarshal(resp.Payload, &setResult)
	if setResult.BookmarkID != "bm_123" {
		t.Errorf("bookmarkId = %q, want %q", setResult.BookmarkID, "bm_123")
	}
	if len(setResult.Tags) != 2 || setResult.Tags[0] != "dev" || setResult.Tags[1] != "go" {
		t.Errorf("tags = %v, want [dev go]", setResult.Tags)
	}
	if setResult.Note != "important bookmark" {
		t.Errorf("note = %q, want %q", setResult.Note, "important bookmark")
	}
	if setResult.Alias != "my-bookmark" {
		t.Errorf("alias = %q, want %q", setResult.Alias, "my-bookmark")
	}

	// Get overlay for same ID → same data back
	resp = request(t, cliW, cliR, "bookmarks.overlay.get", map[string]string{
		"bookmarkId": "bm_123",
	})
	if resp.Type == protocol.TypeError {
		t.Fatalf("overlay.get error: %s", resp.Error.Message)
	}

	var getResult struct {
		BookmarkID string   `json:"bookmarkId"`
		Tags       []string `json:"tags"`
		Note       string   `json:"note"`
		Alias      string   `json:"alias"`
	}
	json.Unmarshal(resp.Payload, &getResult)
	if getResult.BookmarkID != "bm_123" {
		t.Errorf("get bookmarkId = %q, want %q", getResult.BookmarkID, "bm_123")
	}
	if len(getResult.Tags) != 2 {
		t.Errorf("get tags count = %d, want 2", len(getResult.Tags))
	}
	if getResult.Note != "important bookmark" {
		t.Errorf("get note = %q, want %q", getResult.Note, "important bookmark")
	}

	// Get overlay for unknown ID → empty overlay, not error
	resp = request(t, cliW, cliR, "bookmarks.overlay.get", map[string]string{
		"bookmarkId": "unknown_bm",
	})
	if resp.Type == protocol.TypeError {
		t.Fatalf("overlay.get for unknown should not error, got: %s", resp.Error.Message)
	}

	var emptyResult struct {
		BookmarkID string   `json:"bookmarkId"`
		Tags       []string `json:"tags"`
		Note       string   `json:"note"`
	}
	json.Unmarshal(resp.Payload, &emptyResult)
	if emptyResult.BookmarkID != "unknown_bm" {
		t.Errorf("empty overlay bookmarkId = %q, want %q", emptyResult.BookmarkID, "unknown_bm")
	}
	if len(emptyResult.Tags) != 0 {
		t.Errorf("empty overlay tags = %v, want empty", emptyResult.Tags)
	}
	if emptyResult.Note != "" {
		t.Errorf("empty overlay note = %q, want empty", emptyResult.Note)
	}
}

func TestHubBookmarksOverlaySetMissingID(t *testing.T) {
	env := newTestEnv(t)

	_, cliR, cliW := dial(t, env.sockPath)

	resp := request(t, cliW, cliR, "bookmarks.overlay.set", map[string]any{
		"tags": []string{"test"},
		"note": "no id",
	})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error when bookmarkId is missing")
	}
}

func TestHubBookmarksExportNoMirror(t *testing.T) {
	env := newTestEnv(t)

	_, cliR, cliW := dial(t, env.sockPath)

	resp := request(t, cliW, cliR, "bookmarks.export", nil)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error when no mirror exists")
	}
	if !strings.Contains(resp.Error.Message, "no bookmark mirror") {
		t.Errorf("error message = %q, want it to contain 'no bookmark mirror'", resp.Error.Message)
	}
}

// ---------------------------------------------------------------------------
// 7. Search handler tests
// ---------------------------------------------------------------------------

func TestHubSearchSavedCRUD(t *testing.T) {
	env := newTestEnv(t)

	_, cliR, cliW := dial(t, env.sockPath)

	// Create saved search
	resp := request(t, cliW, cliR, "search.saved.create", map[string]any{
		"name": "my-search",
		"query": map[string]any{
			"query":  "golang",
			"scopes": []string{"sessions"},
		},
	})
	if resp.Type == protocol.TypeError {
		t.Fatalf("saved.create error: %s", resp.Error.Message)
	}

	var createResult struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	json.Unmarshal(resp.Payload, &createResult)
	if createResult.Name != "my-search" {
		t.Errorf("name = %q, want %q", createResult.Name, "my-search")
	}
	if !strings.HasPrefix(createResult.ID, "ss_") {
		t.Errorf("id = %q, want prefix ss_", createResult.ID)
	}

	savedID := createResult.ID

	// List saved searches → should contain our search
	resp = request(t, cliW, cliR, "search.saved.list", nil)
	if resp.Type == protocol.TypeError {
		t.Fatalf("saved.list error: %s", resp.Error.Message)
	}

	var listResult struct {
		Searches []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"searches"`
	}
	json.Unmarshal(resp.Payload, &listResult)
	if len(listResult.Searches) != 1 {
		t.Fatalf("searches count = %d, want 1", len(listResult.Searches))
	}
	if listResult.Searches[0].Name != "my-search" {
		t.Errorf("search name = %q, want %q", listResult.Searches[0].Name, "my-search")
	}

	// Delete saved search
	resp = request(t, cliW, cliR, "search.saved.delete", map[string]string{"id": savedID})
	if resp.Type == protocol.TypeError {
		t.Fatalf("saved.delete error: %s", resp.Error.Message)
	}

	// List again → empty
	resp = request(t, cliW, cliR, "search.saved.list", nil)
	json.Unmarshal(resp.Payload, &listResult)
	if len(listResult.Searches) != 0 {
		t.Errorf("searches count after delete = %d, want 0", len(listResult.Searches))
	}
}

func TestHubSearchSavedCreateEmptyName(t *testing.T) {
	env := newTestEnv(t)

	_, cliR, cliW := dial(t, env.sockPath)

	resp := request(t, cliW, cliR, "search.saved.create", map[string]any{
		"name": "",
		"query": map[string]any{
			"query": "test",
		},
	})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for empty name")
	}
}

func TestHubSearchSavedDeleteInvalidID(t *testing.T) {
	env := newTestEnv(t)

	_, cliR, cliW := dial(t, env.sockPath)

	// ID not starting with "ss_"
	resp := request(t, cliW, cliR, "search.saved.delete", map[string]string{"id": "bad_id_123"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for invalid saved search ID")
	}

	// Empty ID
	resp = request(t, cliW, cliR, "search.saved.delete", map[string]string{"id": ""})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for empty saved search ID")
	}
}

func TestHubSearchQuerySessions(t *testing.T) {
	env := newTestEnv(t)

	_, extR, extW := dial(t, env.sockPath)
	sendHello(t, extW, extR)

	go func() {
		for {
			msg, err := extR.Read()
			if err != nil {
				return
			}
			if msg.Type == protocol.TypeRequest {
				var respPayload any
				switch msg.Action {
				case "tabs.list":
					respPayload = map[string]any{
						"tabs": []map[string]any{
							{"id": 1, "windowId": 1, "index": 0, "title": "Work Tab", "url": "https://work-project.com", "active": true, "pinned": false, "groupId": -1},
						},
					}
				case "groups.list":
					respPayload = map[string]any{"groups": []any{}}
				default:
					respPayload = map[string]any{}
				}
				extW.Write(&protocol.Message{
					ID:              msg.ID,
					ProtocolVersion: protocol.ProtocolVersion,
					Type:            protocol.TypeResponse,
					Payload:         mustMarshal(respPayload),
				})
			}
		}
	}()

	_, cliR, cliW := dial(t, env.sockPath)

	// Save a session named "work-project"
	resp := request(t, cliW, cliR, "sessions.save", map[string]string{"name": "work-project"})
	if resp.Type == protocol.TypeError {
		t.Fatalf("save error: %s", resp.Error.Message)
	}

	// Search sessions for "work"
	resp = request(t, cliW, cliR, "search.query", map[string]any{
		"query":  "work",
		"scopes": []string{"sessions"},
	})
	if resp.Type == protocol.TypeError {
		t.Fatalf("search.query error: %s", resp.Error.Message)
	}

	var searchResult struct {
		Results []struct {
			Kind  string `json:"kind"`
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"results"`
		Total int `json:"total"`
	}
	json.Unmarshal(resp.Payload, &searchResult)

	if searchResult.Total == 0 {
		t.Fatal("expected at least 1 search result")
	}

	found := false
	for _, r := range searchResult.Results {
		if r.Kind == "session" && r.ID == "work-project" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find session 'work-project' in results, got %+v", searchResult.Results)
	}
}

// ---------------------------------------------------------------------------
// 8. Workspace handler tests
// ---------------------------------------------------------------------------

func TestHubWorkspaceCRUD(t *testing.T) {
	env := newTestEnv(t)

	_, cliR, cliW := dial(t, env.sockPath)

	// Create
	resp := request(t, cliW, cliR, "workspace.create", map[string]any{
		"name":        "dev-workspace",
		"description": "Development workspace",
		"tags":        []string{"dev", "golang"},
	})
	if resp.Type == protocol.TypeError {
		t.Fatalf("workspace.create error: %s", resp.Error.Message)
	}

	var createResult struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	json.Unmarshal(resp.Payload, &createResult)
	if createResult.Name != "dev-workspace" {
		t.Errorf("name = %q, want %q", createResult.Name, "dev-workspace")
	}
	if !strings.HasPrefix(createResult.ID, "ws_") {
		t.Errorf("id = %q, want prefix ws_", createResult.ID)
	}
	wsID := createResult.ID

	// List → 1 workspace
	resp = request(t, cliW, cliR, "workspace.list", nil)
	var listResult struct {
		Workspaces []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"workspaces"`
	}
	json.Unmarshal(resp.Payload, &listResult)
	if len(listResult.Workspaces) != 1 {
		t.Fatalf("workspaces count = %d, want 1", len(listResult.Workspaces))
	}
	if listResult.Workspaces[0].Name != "dev-workspace" {
		t.Errorf("listed name = %q, want %q", listResult.Workspaces[0].Name, "dev-workspace")
	}

	// Get by ID → full data
	resp = request(t, cliW, cliR, "workspace.get", map[string]string{"id": wsID})
	if resp.Type == protocol.TypeError {
		t.Fatalf("workspace.get error: %s", resp.Error.Message)
	}
	var getResult struct {
		Workspace struct {
			ID          string   `json:"id"`
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Tags        []string `json:"tags"`
		} `json:"workspace"`
	}
	json.Unmarshal(resp.Payload, &getResult)
	if getResult.Workspace.Name != "dev-workspace" {
		t.Errorf("get name = %q, want %q", getResult.Workspace.Name, "dev-workspace")
	}
	if getResult.Workspace.Description != "Development workspace" {
		t.Errorf("get description = %q, want %q", getResult.Workspace.Description, "Development workspace")
	}
	if len(getResult.Workspace.Tags) != 2 {
		t.Errorf("get tags count = %d, want 2", len(getResult.Workspace.Tags))
	}

	// Update (change name)
	newName := "updated-workspace"
	resp = request(t, cliW, cliR, "workspace.update", map[string]any{
		"id":   wsID,
		"name": newName,
	})
	if resp.Type == protocol.TypeError {
		t.Fatalf("workspace.update error: %s", resp.Error.Message)
	}

	// Verify update via get
	resp = request(t, cliW, cliR, "workspace.get", map[string]string{"id": wsID})
	json.Unmarshal(resp.Payload, &getResult)
	if getResult.Workspace.Name != newName {
		t.Errorf("updated name = %q, want %q", getResult.Workspace.Name, newName)
	}

	// Delete
	resp = request(t, cliW, cliR, "workspace.delete", map[string]string{"id": wsID})
	if resp.Type == protocol.TypeError {
		t.Fatalf("workspace.delete error: %s", resp.Error.Message)
	}

	// List → 0 workspaces
	resp = request(t, cliW, cliR, "workspace.list", nil)
	json.Unmarshal(resp.Payload, &listResult)
	if len(listResult.Workspaces) != 0 {
		t.Errorf("workspaces count after delete = %d, want 0", len(listResult.Workspaces))
	}
}

func TestHubWorkspaceCreateEmptyName(t *testing.T) {
	env := newTestEnv(t)

	_, cliR, cliW := dial(t, env.sockPath)

	resp := request(t, cliW, cliR, "workspace.create", map[string]any{
		"name": "",
	})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for empty workspace name")
	}
}

func TestHubWorkspaceGetNotFound(t *testing.T) {
	env := newTestEnv(t)

	_, cliR, cliW := dial(t, env.sockPath)

	resp := request(t, cliW, cliR, "workspace.get", map[string]string{"id": "ws_nonexistent"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for non-existent workspace")
	}
}

// ---------------------------------------------------------------------------
// 9. Sync handler tests
// ---------------------------------------------------------------------------

func TestHubSyncStatus(t *testing.T) {
	env := newTestEnv(t)

	_, cliR, cliW := dial(t, env.sockPath)

	resp := request(t, cliW, cliR, "sync.status", nil)
	// Should not error — the dirs exist (created by newTestEnv)
	if resp.Type == protocol.TypeError {
		t.Fatalf("sync.status error: %s", resp.Error.Message)
	}

	var statusResult struct {
		SyncDir string `json:"syncDir"`
	}
	json.Unmarshal(resp.Payload, &statusResult)
	if statusResult.SyncDir == "" {
		t.Error("syncDir should not be empty")
	}
}

func TestHubSyncRepair(t *testing.T) {
	env := newTestEnv(t)

	_, cliR, cliW := dial(t, env.sockPath)

	resp := request(t, cliW, cliR, "sync.repair", nil)
	if resp.Type == protocol.TypeError {
		t.Fatalf("sync.repair error: %s", resp.Error.Message)
	}

	var repairResult struct {
		Reindexed bool `json:"reindexed"`
	}
	json.Unmarshal(resp.Payload, &repairResult)
	if !repairResult.Reindexed {
		t.Error("expected reindexed = true")
	}
}

// ---------------------------------------------------------------------------
// 10. Unknown action tests
// ---------------------------------------------------------------------------

func TestHubUnknownAction(t *testing.T) {
	env := newTestEnv(t)

	_, cliR, cliW := dial(t, env.sockPath)

	resp := request(t, cliW, cliR, "nonexistent.action", nil)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for unknown action")
	}
	if resp.Error.Code != protocol.ErrUnknownAction {
		t.Errorf("error code = %q, want %q", resp.Error.Code, protocol.ErrUnknownAction)
	}
}

func TestHubUnknownSubAction(t *testing.T) {
	env := newTestEnv(t)

	_, cliR, cliW := dial(t, env.sockPath)

	domains := []string{
		"targets.nonexistent",
		"sessions.nonexistent",
		"collections.nonexistent",
		"bookmarks.nonexistent",
		"search.nonexistent",
		"workspace.nonexistent",
		"sync.nonexistent",
	}

	for _, action := range domains {
		t.Run(action, func(t *testing.T) {
			resp := request(t, cliW, cliR, action, nil)
			if resp.Type != protocol.TypeError {
				t.Fatalf("expected error for %s", action)
			}
			if resp.Error.Code != protocol.ErrUnknownAction {
				t.Errorf("error code = %q, want %q for %s", resp.Error.Code, protocol.ErrUnknownAction, action)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 11. daemon.stop test
// ---------------------------------------------------------------------------

func TestHubDaemonStop(t *testing.T) {
	env := newTestEnv(t)

	_, cliR, cliW := dial(t, env.sockPath)

	resp := request(t, cliW, cliR, "daemon.stop", nil)
	if resp.Type == protocol.TypeError {
		t.Fatalf("daemon.stop error: %s", resp.Error.Message)
	}

	var stopResult struct {
		Stopping bool `json:"stopping"`
	}
	json.Unmarshal(resp.Payload, &stopResult)
	if !stopResult.Stopping {
		t.Error("expected stopping = true")
	}

	// Cancel to let the cleanup goroutine finish promptly
	env.cancel()
	time.Sleep(100 * time.Millisecond)
}

// --- Error injection tests ---

func TestMustMarshal_ChannelFallback(t *testing.T) {
	// channels cannot be marshaled; mustMarshal should return "{}"
	result := mustMarshal(make(chan int))
	if string(result) != "{}" {
		t.Errorf("mustMarshal(chan) = %q, want {}", string(result))
	}
}

func TestMustMarshal_ValidPayload(t *testing.T) {
	result := mustMarshal(map[string]int{"x": 1})
	var m map[string]int
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["x"] != 1 {
		t.Errorf("x = %d, want 1", m["x"])
	}
}

func TestRemoveSubscriber_NoMatch(t *testing.T) {
	h := NewHub(t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "idx.json"))

	// Create a fake subscriber with a dummy conn
	srv, err := net.Listen("unix", filepath.Join(t.TempDir(), "s.sock"))
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srv.Close()

	connCh := make(chan net.Conn, 1)
	go func() {
		c, err := srv.Accept()
		if err == nil {
			connCh <- c
		}
	}()

	clientConn, err := net.Dial("unix", srv.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer clientConn.Close()

	serverConn := <-connCh
	defer serverConn.Close()

	w := protocol.NewWriter(serverConn)
	h.subscribers = append(h.subscribers, &subscriber{
		conn:     serverConn,
		writer:   w,
		patterns: []string{"*"},
	})

	// Remove a different conn — subscriber list should stay the same
	h.removeSubscriber(clientConn)
	if len(h.subscribers) != 1 {
		t.Errorf("subscribers count = %d, want 1 (should not remove unrelated conn)", len(h.subscribers))
	}

	// Remove the actual conn
	h.removeSubscriber(serverConn)
	if len(h.subscribers) != 0 {
		t.Errorf("subscribers count = %d, want 0", len(h.subscribers))
	}
}

func TestUnregisterPending_Idempotent(t *testing.T) {
	h := NewHub(t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "idx.json"))
	// Register then unregister twice — should not panic
	_ = h.registerPendingCh("test-id")
	h.unregisterPending("test-id")
	h.unregisterPending("test-id") // second call should be no-op

	h.pendingMu.Lock()
	_, exists := h.pending["test-id"]
	h.pendingMu.Unlock()
	if exists {
		t.Error("pending should be removed after unregister")
	}
}

func TestHandlePendingResponse_NoMatch(t *testing.T) {
	h := NewHub(t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "idx.json"))
	// handlePendingResponse with unknown ID — should not panic
	msg := &protocol.Message{
		ID:   "unknown-id",
		Type: protocol.TypeResponse,
	}
	h.handlePendingResponse(msg)
	// No assertion needed — just verifying no panic
}

// ===========================================================================
// Helper: mockExtension sets up a target that responds to requests.
// ===========================================================================

type extResponder func(msg *protocol.Message) any

func mockExtension(t *testing.T, sockPath string, responder extResponder) string {
	t.Helper()
	_, extR, extW := dial(t, sockPath)
	targetID := sendHello(t, extW, extR)
	go func() {
		for {
			msg, err := extR.Read()
			if err != nil {
				return
			}
			if msg.Type != protocol.TypeRequest {
				continue
			}
			var payload any
			if responder != nil {
				payload = responder(msg)
			}
			if payload == nil {
				payload = map[string]any{}
			}
			extW.Write(&protocol.Message{
				ID:              msg.ID,
				ProtocolVersion: protocol.ProtocolVersion,
				Type:            protocol.TypeResponse,
				Payload:         mustMarshal(payload),
			})
		}
	}()
	return targetID
}

func envDataDir(env *testEnv) string {
	return filepath.Dir(env.sockPath)
}

// ===========================================================================
// 12. sessionNameFromFile + Collection.Summary
// ===========================================================================

func TestSessionNameFromFile(t *testing.T) {
	tests := []struct{ in, want string }{
		{"work.json", "work"},
		{"my-session.json", "my-session"},
		{"noext", "noext"},
		{".json", ""},
	}
	for _, tt := range tests {
		got := sessionNameFromFile(tt.in)
		if got != tt.want {
			t.Errorf("sessionNameFromFile(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCollectionSummary(t *testing.T) {
	c := &Collection{
		Name:      "test-col",
		CreatedAt: "2025-01-01T00:00:00Z",
		UpdatedAt: "2025-06-01T00:00:00Z",
		Items: []CollectionItem{
			{URL: "https://a.com", Title: "A"},
			{URL: "https://b.com", Title: "B"},
		},
	}
	s := c.Summary()
	if s.Name != "test-col" {
		t.Errorf("Name = %q", s.Name)
	}
	if s.ItemCount != 2 {
		t.Errorf("ItemCount = %d, want 2", s.ItemCount)
	}
}

// ===========================================================================
// 13. unregisterPending (direct)
// ===========================================================================

func TestUnregisterPendingDirect(t *testing.T) {
	h := NewHub("", "", "", "", "", "", "", "")
	ch := h.registerPendingCh("req_1")
	if ch == nil {
		t.Fatal("nil channel")
	}
	h.unregisterPending("req_1")
	h.handlePendingResponse(&protocol.Message{ID: "req_1"})
	h.unregisterPending("nonexistent")
}

// ===========================================================================
// 14. Bookmarks: handleBookmarksTree
// ===========================================================================

func TestHubBookmarksTree(t *testing.T) {
	env := newTestEnv(t)
	dataDir := envDataDir(env)
	mockExtension(t, env.sockPath, func(msg *protocol.Message) any {
		if msg.Action == "bookmarks.tree" {
			return map[string]any{
				"tree": []map[string]any{
					{"id": "1", "title": "Bar", "children": []map[string]any{
						{"id": "2", "title": "Google", "url": "https://google.com"},
					}},
				},
			}
		}
		return nil
	})
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "bookmarks.tree", nil)
	if resp.Type == protocol.TypeError {
		t.Fatalf("error: %s", resp.Error.Message)
	}
	var result struct {
		Tree []struct{ Title string } `json:"tree"`
	}
	json.Unmarshal(resp.Payload, &result)
	if len(result.Tree) == 0 {
		t.Fatal("empty tree")
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(dataDir, "bookmarks", "mirror.json")); err != nil {
		t.Errorf("mirror not created: %v", err)
	}
}

func TestHubBookmarksTreeNoTarget(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "bookmarks.tree", nil)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

// ===========================================================================
// 15. Bookmarks: handleBookmarksSearch
// ===========================================================================

func TestHubBookmarksSearch(t *testing.T) {
	env := newTestEnv(t)
	mockExtension(t, env.sockPath, func(msg *protocol.Message) any {
		if msg.Action == "bookmarks.search" {
			return map[string]any{"results": []map[string]any{{"id": "10", "title": "Hit"}}}
		}
		return nil
	})
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "bookmarks.search", map[string]string{"query": "test"})
	if resp.Type == protocol.TypeError {
		t.Fatalf("error: %s", resp.Error.Message)
	}
}

func TestHubBookmarksSearchNoTarget(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "bookmarks.search", map[string]string{"query": "x"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

// ===========================================================================
// 16. Bookmarks: handleBookmarksGet
// ===========================================================================

func TestHubBookmarksGet(t *testing.T) {
	env := newTestEnv(t)
	mockExtension(t, env.sockPath, func(msg *protocol.Message) any {
		if msg.Action == "bookmarks.get" {
			return map[string]any{"bookmark": map[string]any{"id": "42", "title": "BM"}}
		}
		return nil
	})
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "bookmarks.get", map[string]string{"id": "42"})
	if resp.Type == protocol.TypeError {
		t.Fatalf("error: %s", resp.Error.Message)
	}
}

func TestHubBookmarksGetNoTarget(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "bookmarks.get", map[string]string{"id": "42"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

// ===========================================================================
// 17. Bookmarks: handleBookmarksMirror
// ===========================================================================

func TestHubBookmarksMirrorWithCache(t *testing.T) {
	env := newTestEnv(t)
	dataDir := envDataDir(env)
	mirror := map[string]any{
		"tree": []any{}, "mirroredAt": "2025-01-01T00:00:00Z",
		"targetId": "t1", "nodeCount": 10, "folderCount": 3,
	}
	atomicWriteJSON(filepath.Join(dataDir, "bookmarks"), "mirror.json", mirror)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "bookmarks.mirror", nil)
	if resp.Type == protocol.TypeError {
		t.Fatalf("error: %s", resp.Error.Message)
	}
	var result struct {
		NodeCount   int `json:"nodeCount"`
		FolderCount int `json:"folderCount"`
	}
	json.Unmarshal(resp.Payload, &result)
	if result.NodeCount != 10 {
		t.Errorf("nodeCount = %d, want 10", result.NodeCount)
	}
}

func TestHubBookmarksMirrorFetchFromExt(t *testing.T) {
	env := newTestEnv(t)
	mockExtension(t, env.sockPath, func(msg *protocol.Message) any {
		if msg.Action == "bookmarks.tree" {
			return map[string]any{
				"tree": []map[string]any{
					{"id": "1", "title": "Root", "children": []map[string]any{
						{"id": "2", "title": "Site", "url": "https://site.com"},
					}},
				},
			}
		}
		return nil
	})
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "bookmarks.mirror", nil)
	if resp.Type == protocol.TypeError {
		t.Fatalf("error: %s", resp.Error.Message)
	}
	var result struct {
		NodeCount   int `json:"nodeCount"`
		FolderCount int `json:"folderCount"`
	}
	json.Unmarshal(resp.Payload, &result)
	if result.NodeCount != 2 {
		t.Errorf("nodeCount = %d, want 2", result.NodeCount)
	}
}

func TestHubBookmarksMirrorNoTargetNoCache(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "bookmarks.mirror", nil)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

// ===========================================================================
// 18. updateMirrorFromPayload
// ===========================================================================

func TestUpdateMirrorFromPayload(t *testing.T) {
	dir := t.TempDir()
	h := NewHub("", "", dir, "", "", "", "", "")
	payload := mustMarshal(map[string]any{
		"tree": []map[string]any{
			{"id": "1", "title": "F", "children": []map[string]any{
				{"id": "2", "title": "L", "url": "https://l.com"},
			}},
		},
	})
	result := h.updateMirrorFromPayload(payload, "t1")
	if result == nil {
		t.Fatal("nil result")
	}
	if result.NodeCount != 2 {
		t.Errorf("nodeCount = %d, want 2", result.NodeCount)
	}
	if result.FolderCount != 1 {
		t.Errorf("folderCount = %d, want 1", result.FolderCount)
	}
}

func TestUpdateMirrorFromPayloadBadJSON(t *testing.T) {
	dir := t.TempDir()
	h := NewHub("", "", dir, "", "", "", "", "")
	// mustMarshal a string "hello" (valid JSON but wrong shape for tree)
	result := h.updateMirrorFromPayload(mustMarshal("hello"), "t1")
	if result != nil {
		t.Error("expected nil for wrong shape")
	}
}

func TestUpdateMirrorFromPayloadBadDir(t *testing.T) {
	h := NewHub("", "", "/nonexistent/dir/bookmarks", "", "", "", "", "")
	payload := mustMarshal(map[string]any{
		"tree": []map[string]any{{"id": "1", "title": "X", "url": "https://x.com"}},
	})
	result := h.updateMirrorFromPayload(payload, "t1")
	if result != nil {
		t.Error("expected nil when dir does not exist")
	}
}

// ===========================================================================
// 19. Bookmarks: export with mirror
// ===========================================================================

func TestHubBookmarksExportWithMirror(t *testing.T) {
	env := newTestEnv(t)
	dataDir := envDataDir(env)
	mirror := map[string]any{
		"tree": []map[string]any{
			{"id": "1", "title": "Dev", "children": []map[string]any{
				{"id": "2", "title": "Go Docs", "url": "https://go.dev/doc"},
			}},
		},
		"mirroredAt": "2025-01-01T00:00:00Z", "targetId": "t1",
		"nodeCount": 2, "folderCount": 1,
	}
	atomicWriteJSON(filepath.Join(dataDir, "bookmarks"), "mirror.json", mirror)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "bookmarks.export", nil)
	if resp.Type == protocol.TypeError {
		t.Fatalf("error: %s", resp.Error.Message)
	}
	var result struct{ Content string }
	json.Unmarshal(resp.Payload, &result)
	if !strings.Contains(result.Content, "Go Docs") {
		t.Errorf("content missing 'Go Docs': %s", result.Content)
	}
}

func TestHubBookmarksExportFolderID(t *testing.T) {
	env := newTestEnv(t)
	dataDir := envDataDir(env)
	mirror := map[string]any{
		"tree": []map[string]any{
			{"id": "1", "title": "Root", "children": []map[string]any{
				{"id": "2", "title": "A", "url": "https://a.com"},
			}},
			{"id": "10", "title": "Leaf", "url": "https://leaf.com"},
		},
		"mirroredAt": "2025-01-01T00:00:00Z", "targetId": "t1",
		"nodeCount": 3, "folderCount": 1,
	}
	atomicWriteJSON(filepath.Join(dataDir, "bookmarks"), "mirror.json", mirror)
	_, cliR, cliW := dial(t, env.sockPath)

	// Export specific folder
	resp := request(t, cliW, cliR, "bookmarks.export", map[string]string{"folderId": "1"})
	if resp.Type == protocol.TypeError {
		t.Fatalf("error: %s", resp.Error.Message)
	}
	var result struct{ Content string }
	json.Unmarshal(resp.Payload, &result)
	if !strings.Contains(result.Content, "A") {
		t.Errorf("missing 'A': %s", result.Content)
	}

	// Export leaf node
	resp = request(t, cliW, cliR, "bookmarks.export", map[string]string{"folderId": "10"})
	if resp.Type == protocol.TypeError {
		t.Fatalf("leaf export error: %s", resp.Error.Message)
	}
	json.Unmarshal(resp.Payload, &result)
	if !strings.Contains(result.Content, "Leaf") {
		t.Errorf("missing 'Leaf': %s", result.Content)
	}

	// Non-existent folder
	resp = request(t, cliW, cliR, "bookmarks.export", map[string]string{"folderId": "999"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for missing folder")
	}
}

// ===========================================================================
// 20. Bookmarks overlay: error paths
// ===========================================================================

func TestHubOverlaySetInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "bookmarks.overlay.set", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubOverlayGetInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "bookmarks.overlay.get", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubOverlayGetEmptyID(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "bookmarks.overlay.get", map[string]string{"bookmarkId": ""})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for empty bookmarkId")
	}
}

func TestHubOverlaySetNilTags(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "bookmarks.overlay.set", map[string]any{
		"bookmarkId": "bm_nil", "note": "no tags",
	})
	if resp.Type == protocol.TypeError {
		t.Fatalf("error: %s", resp.Error.Message)
	}
	var result struct{ Tags []string }
	json.Unmarshal(resp.Payload, &result)
	if result.Tags == nil {
		t.Error("tags should not be nil")
	}
}

// ===========================================================================
// 21. Search: searchCollections
// ===========================================================================

func TestHubSearchCollections(t *testing.T) {
	env := newTestEnv(t)
	dataDir := envDataDir(env)
	col := &Collection{
		Name: "frontend-links", CreatedAt: "2025-01-01T00:00:00Z", UpdatedAt: "2025-01-01T00:00:00Z",
		Items: []CollectionItem{
			{URL: "https://react.dev", Title: "React Docs"},
			{URL: "https://vuejs.org", Title: "Vue.js"},
		},
	}
	atomicWriteJSON(filepath.Join(dataDir, "collections"), "frontend-links.json", col)
	_, cliR, cliW := dial(t, env.sockPath)

	// Name match
	resp := request(t, cliW, cliR, "search.query", map[string]any{
		"query": "frontend", "scopes": []string{"collections"},
	})
	if resp.Type == protocol.TypeError {
		t.Fatalf("error: %s", resp.Error.Message)
	}
	var result struct {
		Results []struct {
			Kind string `json:"kind"`
			ID   string `json:"id"`
		} `json:"results"`
		Total int `json:"total"`
	}
	json.Unmarshal(resp.Payload, &result)
	if result.Total == 0 {
		t.Fatal("expected results for 'frontend'")
	}

	// Item title match
	resp = request(t, cliW, cliR, "search.query", map[string]any{
		"query": "react", "scopes": []string{"collections"},
	})
	json.Unmarshal(resp.Payload, &result)
	if result.Total == 0 {
		t.Fatal("expected results for 'react'")
	}

	// Item URL match
	resp = request(t, cliW, cliR, "search.query", map[string]any{
		"query": "vuejs.org", "scopes": []string{"collections"},
	})
	json.Unmarshal(resp.Payload, &result)
	if result.Total == 0 {
		t.Fatal("expected results for 'vuejs.org'")
	}
}

func TestHubSearchCollectionsWithHost(t *testing.T) {
	env := newTestEnv(t)
	dataDir := envDataDir(env)
	col := &Collection{
		Name: "my-col", CreatedAt: "2025-01-01T00:00:00Z", UpdatedAt: "2025-01-01T00:00:00Z",
		Items: []CollectionItem{
			{URL: "https://github.com/foo", Title: "GitHub Foo"},
			{URL: "https://gitlab.com/bar", Title: "GitLab Bar"},
		},
	}
	atomicWriteJSON(filepath.Join(dataDir, "collections"), "my-col.json", col)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "search.query", map[string]any{
		"query": "Git", "scopes": []string{"collections"}, "host": "github.com",
	})
	if resp.Type == protocol.TypeError {
		t.Fatalf("error: %s", resp.Error.Message)
	}
	var result struct {
		Results []struct{ URL string } `json:"results"`
	}
	json.Unmarshal(resp.Payload, &result)
	for _, r := range result.Results {
		if strings.Contains(r.URL, "gitlab") {
			t.Errorf("host filter should exclude gitlab: %s", r.URL)
		}
	}
}

// ===========================================================================
// 22. Search: searchWorkspaces
// ===========================================================================

func TestHubSearchWorkspaces(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	request(t, cliW, cliR, "workspace.create", map[string]any{
		"name": "coding-ws", "tags": []string{"golang", "rust"},
	})

	// By name
	resp := request(t, cliW, cliR, "search.query", map[string]any{
		"query": "coding", "scopes": []string{"workspaces"},
	})
	var result struct {
		Results []struct{ Kind, Title string } `json:"results"`
		Total   int                            `json:"total"`
	}
	json.Unmarshal(resp.Payload, &result)
	if result.Total == 0 {
		t.Fatal("expected ws results for 'coding'")
	}

	// By tag
	resp = request(t, cliW, cliR, "search.query", map[string]any{
		"query": "golang", "scopes": []string{"workspaces"},
	})
	json.Unmarshal(resp.Payload, &result)
	if result.Total == 0 {
		t.Fatal("expected ws results for tag 'golang'")
	}
}

// ===========================================================================
// 23. Search: searchBookmarks
// ===========================================================================

func TestHubSearchBookmarks(t *testing.T) {
	env := newTestEnv(t)
	dataDir := envDataDir(env)
	mirror := map[string]any{
		"tree": []map[string]any{
			{"id": "1", "title": "Go Documentation", "url": "https://go.dev/doc"},
			{"id": "2", "title": "Rust Book", "url": "https://doc.rust-lang.org/book"},
		},
		"mirroredAt": "2025-01-01T00:00:00Z", "targetId": "t1",
		"nodeCount": 2, "folderCount": 0,
	}
	atomicWriteJSON(filepath.Join(dataDir, "bookmarks"), "mirror.json", mirror)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "search.query", map[string]any{
		"query": "Documentation", "scopes": []string{"bookmarks"},
	})
	if resp.Type == protocol.TypeError {
		t.Fatalf("error: %s", resp.Error.Message)
	}
	var result struct {
		Results []struct{ Kind, Title string } `json:"results"`
		Total   int                            `json:"total"`
	}
	json.Unmarshal(resp.Payload, &result)
	if result.Total == 0 {
		t.Fatal("expected bookmark results")
	}
}

func TestHubSearchBookmarksURLMatch(t *testing.T) {
	env := newTestEnv(t)
	dataDir := envDataDir(env)
	mirror := map[string]any{
		"tree": []map[string]any{
			{"id": "1", "title": "Page", "url": "https://uniquesite123.com"},
		},
		"mirroredAt": "2025-01-01T00:00:00Z", "targetId": "t1",
		"nodeCount": 1, "folderCount": 0,
	}
	atomicWriteJSON(filepath.Join(dataDir, "bookmarks"), "mirror.json", mirror)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "search.query", map[string]any{
		"query": "uniquesite123", "scopes": []string{"bookmarks"},
	})
	var result struct {
		Results []struct{ MatchField string `json:"matchField"` } `json:"results"`
		Total   int                                               `json:"total"`
	}
	json.Unmarshal(resp.Payload, &result)
	if result.Total == 0 {
		t.Fatal("expected URL match result")
	}
	if result.Results[0].MatchField != "url" {
		t.Errorf("matchField = %q, want url", result.Results[0].MatchField)
	}
}

func TestHubSearchBookmarksWithHost(t *testing.T) {
	env := newTestEnv(t)
	dataDir := envDataDir(env)
	mirror := map[string]any{
		"tree": []map[string]any{
			{"id": "1", "title": "Go Dev", "url": "https://go.dev"},
			{"id": "2", "title": "Go Proxy", "url": "https://proxy.golang.org"},
		},
		"mirroredAt": "2025-01-01T00:00:00Z", "targetId": "t1",
		"nodeCount": 2, "folderCount": 0,
	}
	atomicWriteJSON(filepath.Join(dataDir, "bookmarks"), "mirror.json", mirror)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "search.query", map[string]any{
		"query": "Go", "scopes": []string{"bookmarks"}, "host": "go.dev",
	})
	var result struct {
		Results []struct{ URL string } `json:"results"`
	}
	json.Unmarshal(resp.Payload, &result)
	for _, r := range result.Results {
		if strings.Contains(r.URL, "proxy.golang") {
			t.Errorf("host filter should exclude proxy.golang.org")
		}
	}
}

// ===========================================================================
// 24. Search: searchTabs
// ===========================================================================

func TestHubSearchTabs(t *testing.T) {
	env := newTestEnv(t)
	mockExtension(t, env.sockPath, func(msg *protocol.Message) any {
		if msg.Action == "tabs.list" {
			return map[string]any{
				"tabs": []map[string]any{
					{"id": 1, "title": "GitHub Dashboard", "url": "https://github.com"},
					{"id": 2, "title": "Google Search", "url": "https://google.com"},
				},
			}
		}
		return nil
	})
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "search.query", map[string]any{
		"query": "GitHub", "scopes": []string{"tabs"},
	})
	if resp.Type == protocol.TypeError {
		t.Fatalf("error: %s", resp.Error.Message)
	}
	var result struct {
		Results []struct{ Kind, Title string } `json:"results"`
		Total   int                            `json:"total"`
	}
	json.Unmarshal(resp.Payload, &result)
	if result.Total == 0 {
		t.Fatal("expected tab results for 'GitHub'")
	}
}

func TestHubSearchTabsURLMatch(t *testing.T) {
	env := newTestEnv(t)
	mockExtension(t, env.sockPath, func(msg *protocol.Message) any {
		if msg.Action == "tabs.list" {
			return map[string]any{
				"tabs": []map[string]any{
					{"id": 1, "title": "Page", "url": "https://specialurl999.io"},
				},
			}
		}
		return nil
	})
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "search.query", map[string]any{
		"query": "specialurl999", "scopes": []string{"tabs"},
	})
	var result struct {
		Results []struct{ MatchField string `json:"matchField"` } `json:"results"`
		Total   int                                               `json:"total"`
	}
	json.Unmarshal(resp.Payload, &result)
	if result.Total == 0 {
		t.Fatal("expected URL match")
	}
	if result.Results[0].MatchField != "url" {
		t.Errorf("matchField = %q, want url", result.Results[0].MatchField)
	}
}

func TestHubSearchTabsWithHost(t *testing.T) {
	env := newTestEnv(t)
	mockExtension(t, env.sockPath, func(msg *protocol.Message) any {
		if msg.Action == "tabs.list" {
			return map[string]any{
				"tabs": []map[string]any{
					{"id": 1, "title": "Go", "url": "https://go.dev"},
					{"id": 2, "title": "Proxy", "url": "https://proxy.golang.org"},
				},
			}
		}
		return nil
	})
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "search.query", map[string]any{
		"query": "Go", "scopes": []string{"tabs"}, "host": "go.dev",
	})
	var result struct {
		Results []struct{ URL string } `json:"results"`
	}
	json.Unmarshal(resp.Payload, &result)
	for _, r := range result.Results {
		if strings.Contains(r.URL, "proxy.golang") {
			t.Errorf("host filter should exclude proxy.golang.org")
		}
	}
}

// ===========================================================================
// 25. sessionContainsHost
// ===========================================================================

func TestSessionContainsHost(t *testing.T) {
	h := NewHub("", "", "", "", "", "", "", "")
	s := Session{Windows: []SessionWindow{{Tabs: []SessionTab{
		{URL: "https://github.com/foo", Title: "GH"},
		{URL: "https://google.com", Title: "G"},
	}}}}
	if !h.sessionContainsHost(s, "github.com") {
		t.Error("expected true for github.com")
	}
	if h.sessionContainsHost(s, "nonexistent.com") {
		t.Error("expected false for nonexistent.com")
	}
	empty := Session{Windows: []SessionWindow{}}
	if h.sessionContainsHost(empty, "x.com") {
		t.Error("expected false for empty session")
	}
}

// ===========================================================================
// 26. Search: all-scope + limit + host on sessions + invalid payload
// ===========================================================================

func TestHubSearchAllScopes(t *testing.T) {
	env := newTestEnv(t)
	dataDir := envDataDir(env)
	mockExtension(t, env.sockPath, func(msg *protocol.Message) any {
		switch msg.Action {
		case "tabs.list":
			return map[string]any{"tabs": []map[string]any{
				{"id": 1, "title": "Universal Tab", "url": "https://universal.com"},
			}}
		case "groups.list":
			return map[string]any{"groups": []any{}}
		}
		return nil
	})
	_, cliR, cliW := dial(t, env.sockPath)
	request(t, cliW, cliR, "sessions.save", map[string]string{"name": "universal-session"})
	request(t, cliW, cliR, "collections.create", map[string]string{"name": "universal-col"})
	request(t, cliW, cliR, "workspace.create", map[string]any{"name": "universal-ws"})
	mirror := map[string]any{
		"tree":        []map[string]any{{"id": "1", "title": "Universal BM", "url": "https://universal-bm.com"}},
		"mirroredAt":  "2025-01-01T00:00:00Z",
		"targetId":    "t1",
		"nodeCount":   1,
		"folderCount": 0,
	}
	atomicWriteJSON(filepath.Join(dataDir, "bookmarks"), "mirror.json", mirror)
	resp := request(t, cliW, cliR, "search.query", map[string]any{"query": "universal"})
	if resp.Type == protocol.TypeError {
		t.Fatalf("error: %s", resp.Error.Message)
	}
	var result struct {
		Results []struct{ Kind string } `json:"results"`
		Total   int                     `json:"total"`
	}
	json.Unmarshal(resp.Payload, &result)
	kinds := map[string]bool{}
	for _, r := range result.Results {
		kinds[r.Kind] = true
	}
	if !kinds["session"] {
		t.Error("missing session in cross-scope")
	}
	if !kinds["bookmark"] {
		t.Error("missing bookmark in cross-scope")
	}
}

func TestHubSearchQueryWithLimit(t *testing.T) {
	env := newTestEnv(t)
	dataDir := envDataDir(env)
	for i := 0; i < 5; i++ {
		s := &Session{Name: fmt.Sprintf("ts-%d", i), CreatedAt: "2025-01-01T00:00:00Z"}
		atomicWriteJSON(filepath.Join(dataDir, "sessions"), fmt.Sprintf("ts-%d.json", i), s)
	}
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "search.query", map[string]any{
		"query": "ts-", "scopes": []string{"sessions"}, "limit": 2,
	})
	var result struct{ Total int }
	json.Unmarshal(resp.Payload, &result)
	if result.Total > 2 {
		t.Errorf("total = %d, want <= 2", result.Total)
	}
}

func TestHubSearchQueryInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "search.query", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for invalid payload")
	}
}

func TestHubSearchSessionsWithHost(t *testing.T) {
	env := newTestEnv(t)
	dataDir := envDataDir(env)
	s := &Session{
		Name: "host-test", CreatedAt: "2025-01-01T00:00:00Z",
		Windows: []SessionWindow{{Tabs: []SessionTab{
			{URL: "https://github.com/proj", Title: "GH"},
		}}},
	}
	atomicWriteJSON(filepath.Join(dataDir, "sessions"), "host-test.json", s)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "search.query", map[string]any{
		"query": "host-test", "scopes": []string{"sessions"}, "host": "github.com",
	})
	var result struct{ Total int }
	json.Unmarshal(resp.Payload, &result)
	if result.Total == 0 {
		t.Error("expected match with github.com host")
	}
	resp = request(t, cliW, cliR, "search.query", map[string]any{
		"query": "host-test", "scopes": []string{"sessions"}, "host": "nonexistent.com",
	})
	json.Unmarshal(resp.Payload, &result)
	if result.Total != 0 {
		t.Errorf("expected 0 for non-matching host, got %d", result.Total)
	}
}

// ===========================================================================
// 27. Workspace: handleWorkspaceSwitch
// ===========================================================================

func TestHubWorkspaceSwitch(t *testing.T) {
	env := newTestEnv(t)
	dataDir := envDataDir(env)
	tabCounter := 0
	mockExtension(t, env.sockPath, func(msg *protocol.Message) any {
		switch msg.Action {
		case "tabs.list":
			return map[string]any{"tabs": []map[string]any{{"id": 100, "title": "Old"}}}
		case "tabs.close":
			return map[string]any{}
		case "tabs.open":
			tabCounter++
			return map[string]any{"tabId": tabCounter}
		case "groups.create":
			return map[string]any{"groupId": 1}
		}
		return nil
	})
	_, cliR, cliW := dial(t, env.sockPath)
	session := &Session{
		Name: "ws-s", CreatedAt: "2025-01-01T00:00:00Z",
		Windows: []SessionWindow{{Tabs: []SessionTab{
			{URL: "https://a.com", Title: "A", Active: true, GroupIndex: 0},
			{URL: "https://b.com", Title: "B", Active: false, GroupIndex: -1},
		}}},
		Groups: []SessionGroup{{Title: "Dev", Color: "blue"}},
	}
	atomicWriteJSON(filepath.Join(dataDir, "sessions"), "ws-s.json", session)
	resp := request(t, cliW, cliR, "workspace.create", map[string]any{
		"name": "sw-ws", "sessions": []string{"ws-s"},
	})
	var cr struct{ ID string `json:"id"` }
	json.Unmarshal(resp.Payload, &cr)

	resp = request(t, cliW, cliR, "workspace.switch", map[string]string{"id": cr.ID})
	if resp.Type == protocol.TypeError {
		t.Fatalf("switch error: %s", resp.Error.Message)
	}
	var sr struct {
		TabsClosed     int `json:"tabsClosed"`
		TabsOpened     int `json:"tabsOpened"`
		WindowsCreated int `json:"windowsCreated"`
	}
	json.Unmarshal(resp.Payload, &sr)
	if sr.TabsClosed != 1 {
		t.Errorf("tabsClosed = %d, want 1", sr.TabsClosed)
	}
	if sr.TabsOpened != 2 {
		t.Errorf("tabsOpened = %d, want 2", sr.TabsOpened)
	}
}

func TestHubWorkspaceSwitchNotFound(t *testing.T) {
	env := newTestEnv(t)
	mockExtension(t, env.sockPath, nil)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "workspace.switch", map[string]string{"id": "ws_none"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for nonexistent workspace")
	}
}

func TestHubWorkspaceSwitchEmptyID(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "workspace.switch", map[string]string{"id": ""})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for empty ID")
	}
}

func TestHubWorkspaceSwitchNoTarget(t *testing.T) {
	env := newTestEnv(t)
	dataDir := envDataDir(env)
	ws := map[string]any{
		"id": "ws_orph", "name": "orphan", "sessions": []string{}, "collections": []string{},
		"tags": []string{}, "status": "active",
		"createdAt": "2025-01-01T00:00:00Z", "updatedAt": "2025-01-01T00:00:00Z",
	}
	atomicWriteJSON(filepath.Join(dataDir, "workspaces"), "ws_orph.json", ws)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "workspace.switch", map[string]string{"id": "ws_orph"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error when no target")
	}
}

func TestHubWorkspaceSwitchInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "workspace.switch", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for invalid payload")
	}
}

func TestHubWorkspaceSwitchEmpty(t *testing.T) {
	env := newTestEnv(t)
	mockExtension(t, env.sockPath, func(msg *protocol.Message) any {
		if msg.Action == "tabs.list" {
			return map[string]any{"tabs": []any{}}
		}
		return nil
	})
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "workspace.create", map[string]any{"name": "empty-ws"})
	var cr struct{ ID string `json:"id"` }
	json.Unmarshal(resp.Payload, &cr)
	resp = request(t, cliW, cliR, "workspace.switch", map[string]string{"id": cr.ID})
	if resp.Type == protocol.TypeError {
		t.Fatalf("error: %s", resp.Error.Message)
	}
	var sr struct{ TabsOpened int `json:"tabsOpened"` }
	json.Unmarshal(resp.Payload, &sr)
	if sr.TabsOpened != 0 {
		t.Errorf("tabsOpened = %d, want 0", sr.TabsOpened)
	}
}

// ===========================================================================
// 28. Workspace: error paths
// ===========================================================================

func TestHubWorkspaceUpdateNotFound(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "workspace.update", map[string]any{"id": "ws_none", "name": "x"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubWorkspaceUpdateEmptyID(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "workspace.update", map[string]any{"id": "", "name": "x"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for empty ID")
	}
}

func TestHubWorkspaceUpdateInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "workspace.update", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubWorkspaceUpdateAllFields(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "workspace.create", map[string]any{"name": "upd-all"})
	var cr struct{ ID string `json:"id"` }
	json.Unmarshal(resp.Payload, &cr)

	resp = request(t, cliW, cliR, "workspace.update", map[string]any{
		"id": cr.ID, "description": "desc", "sessions": []string{"s1"},
		"collections": []string{"c1"}, "bookmarkFolderIds": []string{"bf1"},
		"savedSearchIds": []string{"ss1"}, "tags": []string{"t1"},
		"notes": "notes", "status": "archived", "defaultTarget": "dt1",
	})
	if resp.Type == protocol.TypeError {
		t.Fatalf("update error: %s", resp.Error.Message)
	}
	resp = request(t, cliW, cliR, "workspace.get", map[string]string{"id": cr.ID})
	var gr struct {
		Workspace struct {
			Description       string   `json:"description"`
			Sessions          []string `json:"sessions"`
			Collections       []string `json:"collections"`
			BookmarkFolderIDs []string `json:"bookmarkFolderIds"`
			SavedSearchIDs    []string `json:"savedSearchIds"`
			Tags              []string `json:"tags"`
			Notes             string   `json:"notes"`
			Status            string   `json:"status"`
			DefaultTarget     string   `json:"defaultTarget"`
		} `json:"workspace"`
	}
	json.Unmarshal(resp.Payload, &gr)
	w := gr.Workspace
	if w.Description != "desc" || w.Notes != "notes" || w.Status != "archived" || w.DefaultTarget != "dt1" {
		t.Errorf("field mismatch: %+v", w)
	}
	if len(w.Sessions) != 1 || len(w.Collections) != 1 || len(w.BookmarkFolderIDs) != 1 ||
		len(w.SavedSearchIDs) != 1 || len(w.Tags) != 1 {
		t.Errorf("slice lengths wrong: %+v", w)
	}
}

func TestHubWorkspaceDeleteNotFound(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "workspace.delete", map[string]string{"id": "ws_none"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubWorkspaceDeleteEmptyID(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "workspace.delete", map[string]string{"id": ""})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for empty ID")
	}
}

func TestHubWorkspaceDeleteInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "workspace.delete", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubWorkspaceGetEmptyID(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "workspace.get", map[string]string{"id": ""})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for empty ID")
	}
}

func TestHubWorkspaceGetInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "workspace.get", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubWorkspaceCreateInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "workspace.create", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

// ===========================================================================
// 29. Collections: error paths
// ===========================================================================

func TestHubCollGetInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "collections.get", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubCollGetInvalidName(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "collections.get", map[string]string{"name": "../evil"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for invalid name")
	}
}

func TestHubCollGetNotFound(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "collections.get", map[string]string{"name": "nonexistent"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubCollCreateInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "collections.create", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubCollCreateInvalidName(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "collections.create", map[string]string{"name": "has space"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for invalid name")
	}
}

func TestHubCollDeleteInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "collections.delete", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubCollDeleteInvalidName(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "collections.delete", map[string]string{"name": "has/slash"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for invalid name")
	}
}

func TestHubCollDeleteNotFound(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "collections.delete", map[string]string{"name": "nonexistent"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubCollAddItemsInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "collections.addItems", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubCollAddItemsInvalidName(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "collections.addItems", map[string]any{
		"name": "has space", "items": []map[string]string{{"url": "https://x.com", "title": "X"}},
	})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for invalid name")
	}
}

func TestHubCollAddItemsNotFound(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "collections.addItems", map[string]any{
		"name": "nonexistent", "items": []map[string]string{{"url": "https://x.com", "title": "X"}},
	})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubCollRemoveItemsInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "collections.removeItems", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubCollRemoveItemsInvalidName(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "collections.removeItems", map[string]any{
		"name": "has/slash", "urls": []string{"https://x.com"},
	})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for invalid name")
	}
}

func TestHubCollRemoveItemsNotFound(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "collections.removeItems", map[string]any{
		"name": "nonexistent", "urls": []string{"https://x.com"},
	})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubCollRestoreInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "collections.restore", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubCollRestoreInvalidName(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "collections.restore", map[string]string{"name": "has.dot"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for invalid name")
	}
}

func TestHubCollRestoreNoTarget(t *testing.T) {
	env := newTestEnv(t)
	dataDir := envDataDir(env)
	col := &Collection{
		Name: "orphan-col", CreatedAt: "2025-01-01T00:00:00Z", UpdatedAt: "2025-01-01T00:00:00Z",
		Items: []CollectionItem{{URL: "https://example.com", Title: "Ex"}},
	}
	atomicWriteJSON(filepath.Join(dataDir, "collections"), "orphan-col.json", col)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "collections.restore", map[string]string{"name": "orphan-col"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error when no target")
	}
}

// ===========================================================================
// 30. Sessions: error paths
// ===========================================================================

func TestHubSessGetInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "sessions.get", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubSessGetInvalidName(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "sessions.get", map[string]string{"name": "has space"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for invalid name")
	}
}

func TestHubSessGetNotFound(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "sessions.get", map[string]string{"name": "nonexistent"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubSessSaveInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "sessions.save", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubSessSaveNoTarget(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "sessions.save", map[string]string{"name": "test"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error when no target")
	}
}

func TestHubSessRestoreInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "sessions.restore", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubSessRestoreInvalidName(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "sessions.restore", map[string]string{"name": "has.dot"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for invalid name")
	}
}

func TestHubSessDeleteInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "sessions.delete", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubSessDeleteInvalidName(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "sessions.delete", map[string]string{"name": "has/slash"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for invalid name")
	}
}

func TestHubSessDeleteNotFound(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "sessions.delete", map[string]string{"name": "nonexistent"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

// ===========================================================================
// 31. Subscribe + targets + hello: error paths
// ===========================================================================

func TestHubSubscribeInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "subscribe", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for invalid subscribe payload")
	}
}

func TestHubTargetsDefaultInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "targets.default", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubTargetsLabelInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "targets.label", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubHelloInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, extR, extW := dial(t, env.sockPath)
	msg := &protocol.Message{
		ID: protocol.MakeID(), ProtocolVersion: protocol.ProtocolVersion,
		Type: protocol.TypeHello, Payload: mustMarshal("not-an-object"),
	}
	extW.Write(msg)
	resp, err := extR.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for invalid hello payload")
	}
}

// ===========================================================================
// 32. Search saved: extra error paths
// ===========================================================================

func TestHubSearchSavedDeleteInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "search.saved.delete", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubSearchSavedDeleteNotFound(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "search.saved.delete", map[string]string{"id": "ss_nonexistent"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for non-existent")
	}
}

func TestHubSearchSavedCreateInvalidPayload(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "search.saved.create", 12345)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

// ===========================================================================
// 33. Sync + atomicWriteJSON + forward
// ===========================================================================

func TestSyncLocalDirEdgeCase(t *testing.T) {
	h := NewHub("", "", "", "", "", "", "", "")
	if h.syncLocalDir() != "" {
		t.Errorf("expected empty, got %q", h.syncLocalDir())
	}
}

// ===========================================================================
// Workspace ID path traversal validation
// ===========================================================================

func TestValidateWorkspaceID(t *testing.T) {
	tests := []struct {
		id      string
		wantErr bool
		errMsg  string
	}{
		{"ws_123_1", false, ""},
		{"my-workspace", false, ""},
		{"abc", false, ""},
		{"", true, "id cannot be empty"},
		{"../sessions/foo", true, "invalid characters"},
		{"../../etc/passwd", true, "invalid characters"},
		{"foo/bar", true, "invalid characters"},
		{"foo\\bar", true, "invalid characters"},
		{"foo..bar", true, "invalid characters"},
		{"hello world", true, "invalid characters"},
		{"ws_123.json", true, "invalid characters"},
		{strings.Repeat("a", 129), true, "too long"},
		{strings.Repeat("a", 128), false, ""},
	}
	for _, tt := range tests {
		err := validateWorkspaceID(tt.id)
		if tt.wantErr && err == nil {
			t.Errorf("validateWorkspaceID(%q) = nil, want error containing %q", tt.id, tt.errMsg)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("validateWorkspaceID(%q) = %v, want nil", tt.id, err)
		}
		if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
			t.Errorf("validateWorkspaceID(%q) error = %q, want containing %q", tt.id, err.Error(), tt.errMsg)
		}
	}
}

func TestHubWorkspaceGetPathTraversal(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "workspace.get", map[string]any{
		"id": "../sessions/foo",
	})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for path traversal ID")
	}
	if !strings.Contains(resp.Error.Message, "invalid characters") {
		t.Errorf("error = %q, want invalid characters", resp.Error.Message)
	}
}

func TestHubWorkspaceUpdatePathTraversal(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "workspace.update", map[string]any{
		"id": "../../etc/passwd",
	})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for path traversal ID")
	}
	if !strings.Contains(resp.Error.Message, "invalid characters") {
		t.Errorf("error = %q, want invalid characters", resp.Error.Message)
	}
}

func TestHubWorkspaceDeletePathTraversal(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "workspace.delete", map[string]any{
		"id": "../sessions/secret",
	})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for path traversal ID")
	}
	if !strings.Contains(resp.Error.Message, "invalid characters") {
		t.Errorf("error = %q, want invalid characters", resp.Error.Message)
	}
}

func TestHubWorkspaceSwitchPathTraversal(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "workspace.switch", map[string]any{
		"id": "foo/bar",
	})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for path traversal ID")
	}
	if !strings.Contains(resp.Error.Message, "invalid characters") {
		t.Errorf("error = %q, want invalid characters", resp.Error.Message)
	}
}

func TestAtomicWriteJSONPathTraversal(t *testing.T) {
	dir := t.TempDir()
	err := atomicWriteJSON(dir, "../escape.json", map[string]string{"x": "y"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("error = %q, want path traversal", err.Error())
	}
}

func TestHubForwardNoTarget(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)
	resp := request(t, cliW, cliR, "tabs.list", nil)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
	resp = request(t, cliW, cliR, "groups.list", nil)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error")
	}
}

func TestHubForwardExplicitBadTarget(t *testing.T) {
	env := newTestEnv(t)
	mockExtension(t, env.sockPath, nil)
	_, cliR, cliW := dial(t, env.sockPath)
	msg := &protocol.Message{
		ID: protocol.MakeID(), ProtocolVersion: protocol.ProtocolVersion,
		Type: protocol.TypeRequest, Action: "tabs.list",
		Target: &protocol.TargetSelector{TargetID: "nonexistent"},
	}
	cliW.Write(msg)
	resp, _ := cliR.Read()
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for bad explicit target")
	}
	if resp.Error.Code != protocol.ErrTargetOffline {
		t.Errorf("code = %q, want %q", resp.Error.Code, protocol.ErrTargetOffline)
	}
}

// ===========================================================================
// 34. handleCollectionsList / handleSessionsList / handleWorkspaceList error path
// ===========================================================================

func TestHubCollListDirError(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "ctm-d-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sockPath := filepath.Join(dir, "t.sock")
	lockPath := filepath.Join(dir, "t.lock")
	sessDir := filepath.Join(dir, "sess")
	collDir := filepath.Join(dir, "coll")
	bmDir := filepath.Join(dir, "bm")
	ovDir := filepath.Join(dir, "ov")
	wsDir := filepath.Join(dir, "ws")
	ssDir := filepath.Join(dir, "ss")
	cloudDir := filepath.Join(dir, "cloud")
	os.MkdirAll(sessDir, 0700)
	os.MkdirAll(bmDir, 0700)
	os.MkdirAll(ovDir, 0700)
	os.MkdirAll(wsDir, 0700)
	os.MkdirAll(ssDir, 0700)
	// Do NOT create collDir — this will trigger the error path in handleCollectionsList

	srv := NewServer(sockPath, lockPath, sessDir, collDir, bmDir, ovDir, wsDir, ssDir, cloudDir, filepath.Join(dir, "idx.json"))
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start(ctx) }()
	for i := 0; i < 50; i++ {
		if _, err := net.Dial("unix", sockPath); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Cleanup(func() { cancel(); <-errCh })

	_, cliR, cliW := dial(t, sockPath)

	// collections.list with missing dir — collDir doesn't exist, but listJSONFiles returns nil,nil for that
	// We need to trigger a real error, e.g. a file that can't be read as dir.
	// Actually, listJSONFiles handles os.IsNotExist gracefully. We need a permission error.
	// Let's create the dir and make it unreadable instead.
	os.MkdirAll(collDir, 0700)
	os.Chmod(collDir, 0000)
	t.Cleanup(func() { os.Chmod(collDir, 0700) })

	resp := request(t, cliW, cliR, "collections.list", nil)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for unreadable collections dir")
	}
}

func TestHubSessListDirError(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "ctm-d-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sockPath := filepath.Join(dir, "t.sock")
	lockPath := filepath.Join(dir, "t.lock")
	sessDir := filepath.Join(dir, "sess")
	collDir := filepath.Join(dir, "coll")
	bmDir := filepath.Join(dir, "bm")
	ovDir := filepath.Join(dir, "ov")
	wsDir := filepath.Join(dir, "ws")
	ssDir := filepath.Join(dir, "ss")
	cloudDir := filepath.Join(dir, "cloud")
	os.MkdirAll(sessDir, 0700)
	os.MkdirAll(collDir, 0700)
	os.MkdirAll(bmDir, 0700)
	os.MkdirAll(ovDir, 0700)
	os.MkdirAll(wsDir, 0700)
	os.MkdirAll(ssDir, 0700)

	srv := NewServer(sockPath, lockPath, sessDir, collDir, bmDir, ovDir, wsDir, ssDir, cloudDir, filepath.Join(dir, "idx.json"))
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start(ctx) }()
	for i := 0; i < 50; i++ {
		if _, err := net.Dial("unix", sockPath); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Cleanup(func() { cancel(); <-errCh })

	os.Chmod(sessDir, 0000)
	t.Cleanup(func() { os.Chmod(sessDir, 0700) })

	_, cliR, cliW := dial(t, sockPath)
	resp := request(t, cliW, cliR, "sessions.list", nil)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for unreadable sessions dir")
	}
}

func TestHubWsListDirError(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "ctm-d-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sockPath := filepath.Join(dir, "t.sock")
	lockPath := filepath.Join(dir, "t.lock")
	sessDir := filepath.Join(dir, "sess")
	collDir := filepath.Join(dir, "coll")
	bmDir := filepath.Join(dir, "bm")
	ovDir := filepath.Join(dir, "ov")
	wsDir := filepath.Join(dir, "ws")
	ssDir := filepath.Join(dir, "ss")
	cloudDir := filepath.Join(dir, "cloud")
	os.MkdirAll(sessDir, 0700)
	os.MkdirAll(collDir, 0700)
	os.MkdirAll(bmDir, 0700)
	os.MkdirAll(ovDir, 0700)
	os.MkdirAll(wsDir, 0700)
	os.MkdirAll(ssDir, 0700)

	srv := NewServer(sockPath, lockPath, sessDir, collDir, bmDir, ovDir, wsDir, ssDir, cloudDir, filepath.Join(dir, "idx.json"))
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start(ctx) }()
	for i := 0; i < 50; i++ {
		if _, err := net.Dial("unix", sockPath); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Cleanup(func() { cancel(); <-errCh })

	os.Chmod(wsDir, 0000)
	t.Cleanup(func() { os.Chmod(wsDir, 0700) })

	_, cliR, cliW := dial(t, sockPath)
	resp := request(t, cliW, cliR, "workspace.list", nil)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for unreadable workspaces dir")
	}
}

func TestHubSearchSavedListDirError(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "ctm-d-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sockPath := filepath.Join(dir, "t.sock")
	lockPath := filepath.Join(dir, "t.lock")
	sessDir := filepath.Join(dir, "sess")
	collDir := filepath.Join(dir, "coll")
	bmDir := filepath.Join(dir, "bm")
	ovDir := filepath.Join(dir, "ov")
	wsDir := filepath.Join(dir, "ws")
	ssDir := filepath.Join(dir, "ss")
	cloudDir := filepath.Join(dir, "cloud")
	os.MkdirAll(sessDir, 0700)
	os.MkdirAll(collDir, 0700)
	os.MkdirAll(bmDir, 0700)
	os.MkdirAll(ovDir, 0700)
	os.MkdirAll(wsDir, 0700)
	os.MkdirAll(ssDir, 0700)

	srv := NewServer(sockPath, lockPath, sessDir, collDir, bmDir, ovDir, wsDir, ssDir, cloudDir, filepath.Join(dir, "idx.json"))
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start(ctx) }()
	for i := 0; i < 50; i++ {
		if _, err := net.Dial("unix", sockPath); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Cleanup(func() { cancel(); <-errCh })

	os.Chmod(ssDir, 0000)
	t.Cleanup(func() { os.Chmod(ssDir, 0700) })

	_, cliR, cliW := dial(t, sockPath)
	resp := request(t, cliW, cliR, "search.saved.list", nil)
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for unreadable saved searches dir")
	}
}

// ===========================================================================
// 35. handleBookmarksOverlaySet: write failure path
// ===========================================================================

func TestHubOverlaySetWriteFailure(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "ctm-d-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sockPath := filepath.Join(dir, "t.sock")
	lockPath := filepath.Join(dir, "t.lock")
	sessDir := filepath.Join(dir, "sess")
	collDir := filepath.Join(dir, "coll")
	bmDir := filepath.Join(dir, "bm")
	ovDir := filepath.Join(dir, "ov")
	wsDir := filepath.Join(dir, "ws")
	ssDir := filepath.Join(dir, "ss")
	cloudDir := filepath.Join(dir, "cloud")
	os.MkdirAll(sessDir, 0700)
	os.MkdirAll(collDir, 0700)
	os.MkdirAll(bmDir, 0700)
	os.MkdirAll(ovDir, 0700)
	os.MkdirAll(wsDir, 0700)
	os.MkdirAll(ssDir, 0700)

	srv := NewServer(sockPath, lockPath, sessDir, collDir, bmDir, ovDir, wsDir, ssDir, cloudDir, filepath.Join(dir, "idx.json"))
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start(ctx) }()
	for i := 0; i < 50; i++ {
		if _, err := net.Dial("unix", sockPath); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Cleanup(func() { cancel(); <-errCh })

	// Make overlays dir read-only to trigger write failure
	os.Chmod(ovDir, 0555)
	t.Cleanup(func() { os.Chmod(ovDir, 0700) })

	_, cliR, cliW := dial(t, sockPath)
	resp := request(t, cliW, cliR, "bookmarks.overlay.set", map[string]any{
		"bookmarkId": "bm_fail", "tags": []string{"x"}, "note": "y",
	})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for write failure")
	}
}

// ===========================================================================
// 36. handleCollectionsCreate / handleWorkspaceCreate: write failure path
// ===========================================================================

func TestHubCollCreateWriteFailure(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "ctm-d-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sockPath := filepath.Join(dir, "t.sock")
	lockPath := filepath.Join(dir, "t.lock")
	sessDir := filepath.Join(dir, "sess")
	collDir := filepath.Join(dir, "coll")
	bmDir := filepath.Join(dir, "bm")
	ovDir := filepath.Join(dir, "ov")
	wsDir := filepath.Join(dir, "ws")
	ssDir := filepath.Join(dir, "ss")
	cloudDir := filepath.Join(dir, "cloud")
	os.MkdirAll(sessDir, 0700)
	os.MkdirAll(collDir, 0700)
	os.MkdirAll(bmDir, 0700)
	os.MkdirAll(ovDir, 0700)
	os.MkdirAll(wsDir, 0700)
	os.MkdirAll(ssDir, 0700)

	srv := NewServer(sockPath, lockPath, sessDir, collDir, bmDir, ovDir, wsDir, ssDir, cloudDir, filepath.Join(dir, "idx.json"))
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start(ctx) }()
	for i := 0; i < 50; i++ {
		if _, err := net.Dial("unix", sockPath); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Cleanup(func() { cancel(); <-errCh })

	os.Chmod(collDir, 0555)
	t.Cleanup(func() { os.Chmod(collDir, 0700) })

	_, cliR, cliW := dial(t, sockPath)
	resp := request(t, cliW, cliR, "collections.create", map[string]string{"name": "failcol"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for write failure")
	}
}

func TestHubWsCreateWriteFailure(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "ctm-d-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sockPath := filepath.Join(dir, "t.sock")
	lockPath := filepath.Join(dir, "t.lock")
	sessDir := filepath.Join(dir, "sess")
	collDir := filepath.Join(dir, "coll")
	bmDir := filepath.Join(dir, "bm")
	ovDir := filepath.Join(dir, "ov")
	wsDir := filepath.Join(dir, "ws")
	ssDir := filepath.Join(dir, "ss")
	cloudDir := filepath.Join(dir, "cloud")
	os.MkdirAll(sessDir, 0700)
	os.MkdirAll(collDir, 0700)
	os.MkdirAll(bmDir, 0700)
	os.MkdirAll(ovDir, 0700)
	os.MkdirAll(wsDir, 0700)
	os.MkdirAll(ssDir, 0700)

	srv := NewServer(sockPath, lockPath, sessDir, collDir, bmDir, ovDir, wsDir, ssDir, cloudDir, filepath.Join(dir, "idx.json"))
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start(ctx) }()
	for i := 0; i < 50; i++ {
		if _, err := net.Dial("unix", sockPath); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Cleanup(func() { cancel(); <-errCh })

	os.Chmod(wsDir, 0555)
	t.Cleanup(func() { os.Chmod(wsDir, 0700) })

	_, cliR, cliW := dial(t, sockPath)
	resp := request(t, cliW, cliR, "workspace.create", map[string]any{"name": "failws"})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for write failure")
	}
}

func TestHubSearchSavedCreateWriteFailure(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "ctm-d-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sockPath := filepath.Join(dir, "t.sock")
	lockPath := filepath.Join(dir, "t.lock")
	sessDir := filepath.Join(dir, "sess")
	collDir := filepath.Join(dir, "coll")
	bmDir := filepath.Join(dir, "bm")
	ovDir := filepath.Join(dir, "ov")
	wsDir := filepath.Join(dir, "ws")
	ssDir := filepath.Join(dir, "ss")
	cloudDir := filepath.Join(dir, "cloud")
	os.MkdirAll(sessDir, 0700)
	os.MkdirAll(collDir, 0700)
	os.MkdirAll(bmDir, 0700)
	os.MkdirAll(ovDir, 0700)
	os.MkdirAll(wsDir, 0700)
	os.MkdirAll(ssDir, 0700)

	srv := NewServer(sockPath, lockPath, sessDir, collDir, bmDir, ovDir, wsDir, ssDir, cloudDir, filepath.Join(dir, "idx.json"))
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start(ctx) }()
	for i := 0; i < 50; i++ {
		if _, err := net.Dial("unix", sockPath); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Cleanup(func() { cancel(); <-errCh })

	os.Chmod(ssDir, 0555)
	t.Cleanup(func() { os.Chmod(ssDir, 0700) })

	_, cliR, cliW := dial(t, sockPath)
	resp := request(t, cliW, cliR, "search.saved.create", map[string]any{
		"name": "fail-search", "query": map[string]any{"query": "test"},
	})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for write failure")
	}
}

// ===========================================================================
// Path traversal: bookmarks overlay
// ===========================================================================

func TestHubBookmarksOverlayPathTraversal(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)

	// overlay.set with path traversal bookmarkId
	resp := request(t, cliW, cliR, "bookmarks.overlay.set", map[string]any{
		"bookmarkId": "../evil",
		"tags":       []string{"test"},
		"note":       "traversal attempt",
	})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for path traversal bookmarkId in overlay.set")
	}
	if !strings.Contains(resp.Error.Message, "invalid bookmarkId") {
		t.Errorf("error message = %q, want it to contain 'invalid bookmarkId'", resp.Error.Message)
	}

	// overlay.get with path traversal bookmarkId
	resp = request(t, cliW, cliR, "bookmarks.overlay.get", map[string]string{
		"bookmarkId": "../evil",
	})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for path traversal bookmarkId in overlay.get")
	}
	if !strings.Contains(resp.Error.Message, "invalid bookmarkId") {
		t.Errorf("error message = %q, want it to contain 'invalid bookmarkId'", resp.Error.Message)
	}
}

// ===========================================================================
// Path traversal: search.saved.delete
// ===========================================================================

func TestHubSearchSavedDeletePathTraversal(t *testing.T) {
	env := newTestEnv(t)
	_, cliR, cliW := dial(t, env.sockPath)

	resp := request(t, cliW, cliR, "search.saved.delete", map[string]string{
		"id": "ss_../../../evil",
	})
	if resp.Type != protocol.TypeError {
		t.Fatal("expected error for path traversal in search.saved.delete")
	}
}

// ===========================================================================
// Multi-target search: search.query respects explicit target
// ===========================================================================

func TestSearchQueryTabsRespectsTarget(t *testing.T) {
	env := newTestEnv(t)

	// Register target A: returns tabs with "AlphaTab" title
	_, extAR, extAW := dial(t, env.sockPath)
	targetAID := sendHello(t, extAW, extAR)
	go func() {
		for {
			msg, err := extAR.Read()
			if err != nil {
				return
			}
			if msg.Type == protocol.TypeRequest {
				var payload any
				if msg.Action == "tabs.list" {
					payload = map[string]any{
						"tabs": []map[string]any{
							{"id": 10, "title": "AlphaTab Unique", "url": "https://alpha.example.com"},
						},
					}
				} else {
					payload = map[string]any{}
				}
				extAW.Write(&protocol.Message{
					ID:              msg.ID,
					ProtocolVersion: protocol.ProtocolVersion,
					Type:            protocol.TypeResponse,
					Payload:         mustMarshal(payload),
				})
			}
		}
	}()

	// Register target B: returns tabs with "BetaTab" title
	_, extBR, extBW := dial(t, env.sockPath)
	_ = sendHello(t, extBW, extBR)
	go func() {
		for {
			msg, err := extBR.Read()
			if err != nil {
				return
			}
			if msg.Type == protocol.TypeRequest {
				var payload any
				if msg.Action == "tabs.list" {
					payload = map[string]any{
						"tabs": []map[string]any{
							{"id": 20, "title": "BetaTab Unique", "url": "https://beta.example.com"},
						},
					}
				} else {
					payload = map[string]any{}
				}
				extBW.Write(&protocol.Message{
					ID:              msg.ID,
					ProtocolVersion: protocol.ProtocolVersion,
					Type:            protocol.TypeResponse,
					Payload:         mustMarshal(payload),
				})
			}
		}
	}()

	// Client sends search.query with explicit target A
	_, cliR, cliW := dial(t, env.sockPath)

	msg := &protocol.Message{
		ID:              protocol.MakeID(),
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeRequest,
		Action:          "search.query",
		Target:          &protocol.TargetSelector{TargetID: targetAID},
		Payload: mustMarshal(map[string]any{
			"query":  "Unique",
			"scopes": []string{"tabs"},
		}),
	}
	if err := cliW.Write(msg); err != nil {
		t.Fatalf("write search.query: %v", err)
	}
	resp, err := cliR.Read()
	if err != nil {
		t.Fatalf("read search response: %v", err)
	}
	if resp.Type == protocol.TypeError {
		t.Fatalf("search.query error: %s", resp.Error.Message)
	}

	var result struct {
		Results []struct {
			Kind  string `json:"kind"`
			Title string `json:"title"`
		} `json:"results"`
		Total int `json:"total"`
	}
	json.Unmarshal(resp.Payload, &result)

	if result.Total == 0 {
		t.Fatal("expected search results from target A")
	}

	// Verify all results are from target A (AlphaTab) and none from target B (BetaTab)
	for _, r := range result.Results {
		if strings.Contains(r.Title, "BetaTab") {
			t.Errorf("search should not return BetaTab from target B, got: %q", r.Title)
		}
	}

	foundAlpha := false
	for _, r := range result.Results {
		if strings.Contains(r.Title, "AlphaTab") {
			foundAlpha = true
			break
		}
	}
	if !foundAlpha {
		t.Errorf("search should return AlphaTab from target A, got results: %+v", result.Results)
	}
}

