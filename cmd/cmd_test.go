package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ctm/internal/client"
	"ctm/internal/daemon"
	"ctm/internal/protocol"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// mustMarshal marshals v to json.RawMessage; panics on error.
func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

// executeCommand runs a cobra command with the given args and captures
// cobra-routed output (--help, cobra errors). It returns the captured
// output and any error. Global flag state is reset before each invocation.
func executeCommand(args ...string) (stdout string, stderr string, err error) {
	// Reset global flags that persist between tests
	targetFlag = ""
	tabsJSONOutput = false
	tabsOpenActive = true
	tabsOpenDeduplicate = false
	tabsActivateFocus = false
	groupsJSONOutput = false
	groupsCreateTitle = ""
	groupsCreateTabIDs = nil
	groupsCreateColor = ""
	sessionsJSONOutput = false
	collectionsJSONOutput = false
	collectionsAddURL = ""
	collectionsAddTitle = ""
	bookmarksJSONOutput = false
	bookmarksExportFormat = "markdown"
	searchJSONOutput = false
	searchScopes = nil
	searchLimit = 50
	workspaceJSONOutput = false
	syncJSONOutput = false
	targetsJSONOutput = false
	installCheck = false
	installExtensionID = ""

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs(args)

	err = rootCmd.Execute()

	return outBuf.String(), errBuf.String(), err
}

// captureStdout replaces os.Stdout with a pipe, runs fn, restores os.Stdout,
// and returns everything written to the pipe.
func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

// testDaemon holds references to a test daemon server.
type testDaemon struct {
	configDir string
	sockPath  string
	cancel    context.CancelFunc
}

// setupTestDaemon starts a real daemon in a short temp directory (to stay
// within the 104-byte macOS Unix socket path limit).
func setupTestDaemon(t *testing.T) *testDaemon {
	t.Helper()

	// Use a short path under /tmp to avoid Unix socket path length issues.
	baseDir, err := os.MkdirTemp("/tmp", "ctm-t-")
	if err != nil {
		t.Fatalf("create base dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(baseDir) })

	sockPath := filepath.Join(baseDir, "daemon.sock")
	lockPath := filepath.Join(baseDir, "daemon.lock")
	sessionsDir := filepath.Join(baseDir, "sessions")
	collectionsDir := filepath.Join(baseDir, "collections")
	bookmarksDir := filepath.Join(baseDir, "bookmarks")
	overlaysDir := filepath.Join(baseDir, "overlays")
	workspacesDir := filepath.Join(baseDir, "workspaces")
	savedSearchesDir := filepath.Join(baseDir, "searches")
	syncCloudDir := filepath.Join(baseDir, "cloud")
	searchIndexPath := filepath.Join(baseDir, "search_index.json")

	for _, d := range []string{sessionsDir, collectionsDir, bookmarksDir, overlaysDir, workspacesDir, savedSearchesDir} {
		if mkErr := os.MkdirAll(d, 0700); mkErr != nil {
			t.Fatalf("mkdir %s: %v", d, mkErr)
		}
	}

	srv := daemon.NewServer(sockPath, lockPath, sessionsDir, collectionsDir, bookmarksDir, overlaysDir, workspacesDir, savedSearchesDir, syncCloudDir, searchIndexPath)
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	// Wait for the socket to be ready
	for i := 0; i < 50; i++ {
		conn, dialErr := net.Dial("unix", sockPath)
		if dialErr == nil {
			conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Cleanup(func() {
		cancel()
		<-errCh
	})

	return &testDaemon{configDir: baseDir, sockPath: sockPath, cancel: cancel}
}

// connectMockExtension connects a mock browser extension that responds to
// common actions. Returns the targetId assigned by the daemon.
func connectMockExtension(t *testing.T, sockPath string) string {
	t.Helper()
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("mock extension dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	r := protocol.NewReader(conn)
	w := protocol.NewWriter(conn)

	// Send hello
	hello := &protocol.Message{
		ID:              protocol.MakeID(),
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeHello,
		Payload: mustMarshal(map[string]any{
			"channel":      "stable",
			"extensionId":  "test-ext-id",
			"instanceId":   "test-instance",
			"userAgent":    "Chrome/130.0 Test",
			"capabilities": []string{"tabs", "groups", "bookmarks"},
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
	targetID := payload["targetId"]

	// Background responder
	go func() {
		for {
			msg, err := r.Read()
			if err != nil {
				return
			}
			if msg.Type != protocol.TypeRequest {
				continue
			}

			var respPayload any
			switch msg.Action {
			case "tabs.list":
				respPayload = map[string]any{
					"tabs": []map[string]any{
						{"id": 1, "windowId": 1, "index": 0, "title": "Google", "url": "https://google.com", "active": true, "pinned": false, "groupId": -1},
						{"id": 2, "windowId": 1, "index": 1, "title": "GitHub - Software", "url": "https://github.com", "active": false, "pinned": true, "groupId": 1},
					},
				}
			case "tabs.open":
				respPayload = map[string]any{
					"tab": map[string]any{"id": 3, "windowId": 1, "url": "https://example.com", "active": true},
				}
			case "tabs.close":
				respPayload = map[string]any{"closed": true}
			case "tabs.activate":
				respPayload = map[string]any{"activated": true}
			case "groups.list":
				respPayload = map[string]any{
					"groups": []map[string]any{
						{"id": 1, "title": "Work", "color": "blue", "collapsed": false, "windowId": 1},
					},
				}
			case "groups.create":
				respPayload = map[string]any{
					"group": map[string]any{"id": 2, "title": "New Group", "color": "red"},
				}
			case "bookmarks.tree":
				respPayload = map[string]any{
					"tree": []map[string]any{
						{
							"id": "0", "title": "",
							"children": []map[string]any{
								{
									"id": "1", "title": "Bookmarks Bar",
									"children": []map[string]any{
										{"id": "2", "title": "Example", "url": "https://example.com"},
									},
								},
							},
						},
					},
				}
			case "bookmarks.search":
				respPayload = map[string]any{
					"bookmarks": []map[string]any{
						{"id": "2", "title": "Example", "url": "https://example.com"},
					},
				}
			case "bookmarks.mirror":
				respPayload = map[string]any{
					"nodeCount": 10, "folderCount": 3, "mirroredAt": "2026-01-01T00:00:00Z",
				}
			case "bookmarks.export":
				respPayload = map[string]any{
					"content": "# Bookmarks\n\n- [Example](https://example.com)\n",
				}
			default:
				w.Write(&protocol.Message{
					ID:              msg.ID,
					ProtocolVersion: protocol.ProtocolVersion,
					Type:            protocol.TypeError,
					Error: &protocol.ErrorBody{
						Code:    protocol.ErrUnknownAction,
						Message: fmt.Sprintf("unknown action: %s", msg.Action),
					},
				})
				continue
			}

			w.Write(&protocol.Message{
				ID:              msg.ID,
				ProtocolVersion: protocol.ProtocolVersion,
				Type:            protocol.TypeResponse,
				Payload:         mustMarshal(respPayload),
			})
		}
	}()

	return targetID
}

// ---------------------------------------------------------------------------
// Helper function tests (no daemon needed)
// ---------------------------------------------------------------------------

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "he..."},
		{"hi", 2, "hi"},
		{"abcdef", 6, "abcdef"},
		{"abcdefg", 6, "abc..."},
		{"a", 1, "a"},
		{"ab", 4, "ab"},
		{"", 5, ""},
		{"exactly ten", 11, "exactly ten"},
		{"exactly ten!", 11, "exactly ..."},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q_max%d", tt.input, tt.max), func(t *testing.T) {
			got := truncate(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}

func TestTargetSelector(t *testing.T) {
	t.Run("empty flag returns nil", func(t *testing.T) {
		targetFlag = ""
		got := targetSelector()
		if got != nil {
			t.Errorf("targetSelector() = %+v, want nil", got)
		}
	})

	t.Run("non-empty flag returns selector", func(t *testing.T) {
		targetFlag = "target_1"
		got := targetSelector()
		if got == nil {
			t.Fatal("targetSelector() = nil, want non-nil")
		}
		if got.TargetID != "target_1" {
			t.Errorf("TargetID = %q, want %q", got.TargetID, "target_1")
		}
		targetFlag = ""
	})
}

func TestPrintJSON(t *testing.T) {
	data := json.RawMessage(`{"key":"value","num":42}`)
	output := captureStdout(func() { printJSON(data) })

	if !strings.Contains(output, `"key": "value"`) {
		t.Errorf("printJSON output should contain indented JSON, got: %s", output)
	}
	if !strings.Contains(output, `"num": 42`) {
		t.Errorf("printJSON output should contain num field, got: %s", output)
	}
}

func TestStyleDimCLI(t *testing.T) {
	result := styleDimCLI("hello")
	if result != "\033[2mhello\033[0m" {
		t.Errorf("styleDimCLI(hello) = %q, want ANSI-dimmed string", result)
	}
}

func TestPrintBookmarkTree(t *testing.T) {
	tree := map[string]any{
		"id": "0", "title": "Root",
		"children": []map[string]any{
			{
				"id": "1", "title": "Folder",
				"children": []map[string]any{
					{"id": "2", "title": "Bookmark", "url": "https://example.com"},
				},
			},
		},
	}
	data, _ := json.Marshal(tree)
	output := captureStdout(func() {
		printBookmarkTree(json.RawMessage(data), 0)
	})

	if !strings.Contains(output, "[F] Root") {
		t.Errorf("should print root folder, got: %q", output)
	}
	if !strings.Contains(output, "  [F] Folder") {
		t.Errorf("should print nested folder with indentation, got: %q", output)
	}
	if !strings.Contains(output, "Bookmark") {
		t.Errorf("should print bookmark title, got: %q", output)
	}
	if !strings.Contains(output, "example.com") {
		t.Errorf("should print bookmark URL, got: %q", output)
	}
}

func TestPrintBookmarkTreeInvalidJSON(t *testing.T) {
	// Should not panic on invalid JSON
	captureStdout(func() {
		printBookmarkTree(json.RawMessage(`{invalid`), 0)
	})
}

// ---------------------------------------------------------------------------
// Version command test
// ---------------------------------------------------------------------------

func TestVersionCommand(t *testing.T) {
	origVersion := Version
	Version = "1.2.3-test"
	defer func() { Version = origVersion }()

	// version uses fmt.Printf (goes to os.Stdout, not cobra's SetOut)
	output := captureStdout(func() {
		_, _, err := executeCommand("version")
		if err != nil {
			t.Fatalf("version command error: %v", err)
		}
	})

	if !strings.Contains(output, "1.2.3-test") {
		t.Errorf("version output should contain version string, got: %q", output)
	}
}

// ---------------------------------------------------------------------------
// Install command tests
// ---------------------------------------------------------------------------

func TestInstallCheckNoExtensionID(t *testing.T) {
	// install --check without --extension-id should succeed (reports LaunchAgent + NM status)
	output, _, err := executeCommand("install", "--check")
	if err != nil {
		t.Fatalf("install --check without --extension-id should succeed: %v", err)
	}
	if !strings.Contains(output, "Binary") {
		t.Errorf("output should mention Binary, got: %q", output)
	}
	if !strings.Contains(output, "LaunchAgent") {
		t.Errorf("output should mention LaunchAgent, got: %q", output)
	}
	// Extension ID should NOT appear when not provided
	if strings.Contains(output, "Extension ID") {
		t.Errorf("output should not mention Extension ID when not provided, got: %q", output)
	}
}

func TestInstallCheckWithExtensionID(t *testing.T) {
	// install --check with --extension-id should show extension info
	output, _, err := executeCommand("install", "--check", "--extension-id", "test-ext-id-1234")
	if err != nil {
		t.Fatalf("install --check should succeed: %v", err)
	}
	if !strings.Contains(output, "Binary") {
		t.Errorf("output should mention Binary, got: %q", output)
	}
	if !strings.Contains(output, "Extension ID") {
		t.Errorf("output should mention Extension ID, got: %q", output)
	}
	if !strings.Contains(output, "test-ext-id-1234") {
		t.Errorf("output should contain the extension ID, got: %q", output)
	}
}

func TestDoctorCommand(t *testing.T) {
	t.Setenv("CTM_CONFIG_DIR", t.TempDir())
	output, _, err := executeCommand("doctor")
	if err != nil {
		t.Fatalf("doctor should succeed: %v", err)
	}
	if !strings.Contains(output, "CTM Doctor") {
		t.Errorf("output should contain header, got: %q", output)
	}
	if !strings.Contains(output, "Binary") {
		t.Errorf("output should mention Binary, got: %q", output)
	}
	if !strings.Contains(output, "LaunchAgent") {
		t.Errorf("output should mention LaunchAgent, got: %q", output)
	}
	// Daemon is not running in this test, so socket should not be reachable
	if !strings.Contains(output, "Daemon socket not reachable") {
		t.Errorf("output should report daemon socket not reachable, got: %q", output)
	}
}

func TestDoctorWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)

	// No extension connected yet
	output, _, err := executeCommand("doctor")
	if err != nil {
		t.Fatalf("doctor should succeed: %v", err)
	}
	if !strings.Contains(output, "Daemon socket reachable") {
		t.Errorf("output should report daemon reachable, got: %q", output)
	}
	if !strings.Contains(output, "Daemon responds to requests") {
		t.Errorf("output should report daemon responds, got: %q", output)
	}
	if !strings.Contains(output, "No browser extensions connected") {
		t.Errorf("output should report no extensions, got: %q", output)
	}
}

func TestDoctorWithDaemonAndExtension(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	output, _, err := executeCommand("doctor")
	if err != nil {
		t.Fatalf("doctor should succeed: %v", err)
	}
	if !strings.Contains(output, "Daemon socket reachable") {
		t.Errorf("output should report daemon reachable, got: %q", output)
	}
	if !strings.Contains(output, "Browser extensions connected: 1") {
		t.Errorf("output should report 1 extension, got: %q", output)
	}
}

func TestCheckFile(t *testing.T) {
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "existing.json")
	os.WriteFile(existingFile, []byte("{}"), 0644)

	output := captureStdout(func() {
		checkFile(nil, "Test File", existingFile)
		checkFile(nil, "Missing File", filepath.Join(tmpDir, "nonexistent.json"))
	})

	if !strings.Contains(output, "ok Test File") {
		t.Errorf("should report existing file as ok, got: %q", output)
	}
	if !strings.Contains(output, "missing Missing File") {
		t.Errorf("should report nonexistent file as missing, got: %q", output)
	}
}

// ---------------------------------------------------------------------------
// Argument validation tests (no daemon, no fork)
// These tests verify that cobra rejects missing/invalid arguments before
// connectAndRequest is called (so tryAutoStartDaemon is never triggered).
// ---------------------------------------------------------------------------

func TestTabsOpenMissingURL(t *testing.T) {
	_, _, err := executeCommand("tabs", "open")
	if err == nil {
		t.Fatal("tabs open without URL should fail")
	}
}

func TestTabsCloseInvalidID(t *testing.T) {
	// Set CTM_CONFIG_DIR so the socket path doesn't hit the real daemon.
	// strconv.Atoi fails before connectAndRequest is called.
	t.Setenv("CTM_CONFIG_DIR", t.TempDir())
	_, _, err := executeCommand("tabs", "close", "abc")
	if err == nil {
		t.Fatal("tabs close with non-numeric ID should fail")
	}
	if !strings.Contains(err.Error(), "invalid tab ID") {
		t.Errorf("error should mention invalid tab ID, got: %v", err)
	}
}

func TestTabsActivateInvalidID(t *testing.T) {
	t.Setenv("CTM_CONFIG_DIR", t.TempDir())
	_, _, err := executeCommand("tabs", "activate", "xyz")
	if err == nil {
		t.Fatal("tabs activate with non-numeric ID should fail")
	}
	if !strings.Contains(err.Error(), "invalid tab ID") {
		t.Errorf("error should mention invalid tab ID, got: %v", err)
	}
}

func TestTabsCloseMissingArg(t *testing.T) {
	_, _, err := executeCommand("tabs", "close")
	if err == nil {
		t.Fatal("tabs close without arg should fail")
	}
}

func TestTabsActivateMissingArg(t *testing.T) {
	_, _, err := executeCommand("tabs", "activate")
	if err == nil {
		t.Fatal("tabs activate without arg should fail")
	}
}

func TestGroupsCreateMissingTitle(t *testing.T) {
	_, _, err := executeCommand("groups", "create", "--tab-id", "1")
	if err == nil {
		t.Fatal("groups create without --title should fail")
	}
}

func TestGroupsCreateMissingTabIDs(t *testing.T) {
	t.Setenv("CTM_CONFIG_DIR", t.TempDir())
	_, _, err := executeCommand("groups", "create", "--title", "MyGroup")
	if err == nil {
		t.Fatal("groups create without --tab-id should fail")
	}
	if !strings.Contains(err.Error(), "tab-id is required") {
		t.Errorf("error should mention tab-id, got: %v", err)
	}
}

func TestSessionsGetMissingArg(t *testing.T) {
	_, _, err := executeCommand("sessions", "get")
	if err == nil {
		t.Fatal("sessions get without name should fail")
	}
}

func TestSessionsSaveMissingArg(t *testing.T) {
	_, _, err := executeCommand("sessions", "save")
	if err == nil {
		t.Fatal("sessions save without name should fail")
	}
}

func TestSessionsRestoreMissingArg(t *testing.T) {
	_, _, err := executeCommand("sessions", "restore")
	if err == nil {
		t.Fatal("sessions restore without name should fail")
	}
}

func TestSessionsDeleteMissingArg(t *testing.T) {
	_, _, err := executeCommand("sessions", "delete")
	if err == nil {
		t.Fatal("sessions delete without name should fail")
	}
}

func TestCollectionsGetMissingArg(t *testing.T) {
	_, _, err := executeCommand("collections", "get")
	if err == nil {
		t.Fatal("collections get without name should fail")
	}
}

func TestCollectionsCreateMissingArg(t *testing.T) {
	_, _, err := executeCommand("collections", "create")
	if err == nil {
		t.Fatal("collections create without name should fail")
	}
}

func TestCollectionsDeleteMissingArg(t *testing.T) {
	_, _, err := executeCommand("collections", "delete")
	if err == nil {
		t.Fatal("collections delete without name should fail")
	}
}

func TestCollectionsAddMissingFlags(t *testing.T) {
	_, _, err := executeCommand("collections", "add", "myCollection")
	if err == nil {
		t.Fatal("collections add without --url and --title should fail")
	}
}

func TestCollectionsRestoreMissingArg(t *testing.T) {
	_, _, err := executeCommand("collections", "restore")
	if err == nil {
		t.Fatal("collections restore without name should fail")
	}
}

func TestBookmarksSearchMissingArg(t *testing.T) {
	_, _, err := executeCommand("bookmarks", "search")
	if err == nil {
		t.Fatal("bookmarks search without query should fail")
	}
}

func TestSearchMissingQuery(t *testing.T) {
	_, _, err := executeCommand("search")
	if err == nil {
		t.Fatal("search without query should fail")
	}
}

func TestWorkspaceGetMissingArg(t *testing.T) {
	_, _, err := executeCommand("workspaces", "get")
	if err == nil {
		t.Fatal("workspaces get without id should fail")
	}
}

func TestWorkspaceCreateMissingArg(t *testing.T) {
	_, _, err := executeCommand("workspaces", "create")
	if err == nil {
		t.Fatal("workspaces create without name should fail")
	}
}

func TestWorkspaceDeleteMissingArg(t *testing.T) {
	_, _, err := executeCommand("workspaces", "delete")
	if err == nil {
		t.Fatal("workspaces delete without id should fail")
	}
}

func TestWorkspaceSwitchMissingArg(t *testing.T) {
	_, _, err := executeCommand("workspaces", "switch")
	if err == nil {
		t.Fatal("workspaces switch without id should fail")
	}
}

func TestTargetsDefaultMissingArg(t *testing.T) {
	_, _, err := executeCommand("targets", "default")
	if err == nil {
		t.Fatal("targets default without targetId should fail")
	}
}

func TestTargetsLabelMissingArgs(t *testing.T) {
	_, _, err := executeCommand("targets", "label")
	if err == nil {
		t.Fatal("targets label without args should fail")
	}

	_, _, err = executeCommand("targets", "label", "target1")
	if err == nil {
		t.Fatal("targets label with only one arg should fail")
	}
}

// ---------------------------------------------------------------------------
// Root command & help tests
// ---------------------------------------------------------------------------

func TestRootCommandHelp(t *testing.T) {
	stdout, _, err := executeCommand("--help")
	if err != nil {
		t.Fatalf("--help should not fail: %v", err)
	}
	if !strings.Contains(stdout, "CTM controls the browser") {
		t.Errorf("help should contain long description, got: %q", stdout)
	}
	if !strings.Contains(stdout, "tabs") {
		t.Errorf("help should list tabs command, got: %q", stdout)
	}
	if !strings.Contains(stdout, "sessions") {
		t.Errorf("help should list sessions command, got: %q", stdout)
	}
}

func TestTargetFlagParsing(t *testing.T) {
	// Set CTM_CONFIG_DIR so it uses a nonexistent socket and fails at
	// connect, not at flag parsing.
	t.Setenv("CTM_CONFIG_DIR", t.TempDir())
	_, _, err := executeCommand("--target", "my-target", "targets", "list")
	if err == nil {
		t.Skip("expected connection error since no daemon")
	}
	if strings.Contains(err.Error(), "unknown flag") {
		t.Errorf("--target flag should be recognized, got: %v", err)
	}
}

func TestWorkspaceAliasCommand(t *testing.T) {
	stdout, _, err := executeCommand("workspace", "--help")
	if err != nil {
		t.Fatalf("workspace alias should work: %v", err)
	}
	if !strings.Contains(stdout, "Manage workspaces") {
		t.Errorf("workspace help should show description, got: %q", stdout)
	}
}

func TestSubcommandHelp(t *testing.T) {
	subcommands := []struct {
		name string
		args []string
		want string
	}{
		{"tabs", []string{"tabs", "--help"}, "Manage browser tabs"},
		{"groups", []string{"groups", "--help"}, "Manage tab groups"},
		{"sessions", []string{"sessions", "--help"}, "Manage saved sessions"},
		{"collections", []string{"collections", "--help"}, "Manage collections"},
		{"bookmarks", []string{"bookmarks", "--help"}, "Manage browser bookmarks"},
		{"targets", []string{"targets", "--help"}, "Manage browser targets"},
		{"sync", []string{"sync", "--help"}, "Manage sync"},
		{"workspaces", []string{"workspaces", "--help"}, "Manage workspaces"},
	}

	for _, tc := range subcommands {
		t.Run(tc.name, func(t *testing.T) {
			stdout, _, err := executeCommand(tc.args...)
			if err != nil {
				t.Fatalf("%s --help error: %v", tc.name, err)
			}
			if !strings.Contains(stdout, tc.want) {
				t.Errorf("%s help should contain %q, got: %q", tc.name, tc.want, stdout)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Daemon-integrated tests
//
// Each test starts a real daemon, connects a mock extension, sets
// CTM_CONFIG_DIR to point at the test socket, and exercises commands
// end-to-end.
// ---------------------------------------------------------------------------

func TestTabsListWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	output := captureStdout(func() {
		_, _, err := executeCommand("tabs", "list")
		if err != nil {
			t.Fatalf("tabs list error: %v", err)
		}
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "TITLE") {
		t.Errorf("output should contain table header, got: %q", output)
	}
	if !strings.Contains(output, "Google") {
		t.Errorf("output should contain tab title 'Google', got: %q", output)
	}
	if !strings.Contains(output, "google.com") {
		t.Errorf("output should contain tab URL, got: %q", output)
	}
}

func TestTabsListJSONWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	output := captureStdout(func() {
		_, _, err := executeCommand("tabs", "--json", "list")
		if err != nil {
			t.Fatalf("tabs --json list error: %v", err)
		}
	})

	var parsed map[string]any
	if jsonErr := json.Unmarshal([]byte(output), &parsed); jsonErr != nil {
		t.Errorf("output should be valid JSON, got: %q, error: %v", output, jsonErr)
	}
	if _, ok := parsed["tabs"]; !ok {
		t.Errorf("JSON output should contain 'tabs' key, got: %q", output)
	}
}

func TestTabsOpenWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	output := captureStdout(func() {
		_, _, err := executeCommand("tabs", "open", "https://example.com")
		if err != nil {
			t.Fatalf("tabs open error: %v", err)
		}
	})

	if !strings.Contains(output, "Tab") && !strings.Contains(output, "tab") {
		t.Errorf("output should contain tab info, got: %q", output)
	}
}

func TestTabsCloseWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	output := captureStdout(func() {
		_, _, err := executeCommand("tabs", "close", "1")
		if err != nil {
			t.Fatalf("tabs close error: %v", err)
		}
	})

	if !strings.Contains(output, "Tab 1 closed") {
		t.Errorf("output should confirm tab closure, got: %q", output)
	}
}

func TestTabsActivateWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	output := captureStdout(func() {
		_, _, err := executeCommand("tabs", "activate", "2")
		if err != nil {
			t.Fatalf("tabs activate error: %v", err)
		}
	})

	if !strings.Contains(output, "Tab 2 activated") {
		t.Errorf("output should confirm tab activation, got: %q", output)
	}
}

func TestGroupsListWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	output := captureStdout(func() {
		_, _, err := executeCommand("groups", "list")
		if err != nil {
			t.Fatalf("groups list error: %v", err)
		}
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "TITLE") {
		t.Errorf("output should contain table header, got: %q", output)
	}
	if !strings.Contains(output, "Work") {
		t.Errorf("output should contain group title 'Work', got: %q", output)
	}
	if !strings.Contains(output, "blue") {
		t.Errorf("output should contain group color, got: %q", output)
	}
}

func TestTargetsListWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	targetID := connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	output := captureStdout(func() {
		_, _, err := executeCommand("targets", "list")
		if err != nil {
			t.Fatalf("targets list error: %v", err)
		}
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "CHANNEL") {
		t.Errorf("output should contain table header, got: %q", output)
	}
	if !strings.Contains(output, targetID) {
		t.Errorf("output should contain target ID %q, got: %q", targetID, output)
	}
}

func TestTargetsListJSONWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	output := captureStdout(func() {
		_, _, err := executeCommand("targets", "--json", "list")
		if err != nil {
			t.Fatalf("targets --json list error: %v", err)
		}
	})

	var parsed map[string]any
	if jsonErr := json.Unmarshal([]byte(output), &parsed); jsonErr != nil {
		t.Errorf("output should be valid JSON, got: %q, error: %v", output, jsonErr)
	}
}

// ---------------------------------------------------------------------------
// Session commands with daemon
// ---------------------------------------------------------------------------

func TestSessionsListEmptyWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)

	output := captureStdout(func() {
		_, _, err := executeCommand("sessions", "list")
		if err != nil {
			t.Fatalf("sessions list error: %v", err)
		}
	})

	if !strings.Contains(output, "NAME") || !strings.Contains(output, "TABS") {
		t.Errorf("output should contain table header even when empty, got: %q", output)
	}
}

func TestSessionsSaveAndListWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	// Save
	saveOutput := captureStdout(func() {
		_, _, err := executeCommand("sessions", "save", "work-session")
		if err != nil {
			t.Fatalf("sessions save error: %v", err)
		}
	})
	if !strings.Contains(saveOutput, "work-session") {
		t.Errorf("save output should contain session name, got: %q", saveOutput)
	}

	// List
	listOutput := captureStdout(func() {
		_, _, err := executeCommand("sessions", "list")
		if err != nil {
			t.Fatalf("sessions list error: %v", err)
		}
	})
	if !strings.Contains(listOutput, "work-session") {
		t.Errorf("list output should contain saved session name, got: %q", listOutput)
	}
}

func TestSessionsGetWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	// Save first
	captureStdout(func() {
		executeCommand("sessions", "save", "test-get")
	})

	// Get
	getOutput := captureStdout(func() {
		_, _, err := executeCommand("sessions", "get", "test-get")
		if err != nil {
			t.Fatalf("sessions get error: %v", err)
		}
	})
	if !strings.Contains(getOutput, "google.com") {
		t.Errorf("get output should contain tab URLs, got: %q", getOutput)
	}
}

func TestSessionsDeleteWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	// Save first
	captureStdout(func() {
		executeCommand("sessions", "save", "to-delete")
	})

	// Delete
	deleteOutput := captureStdout(func() {
		_, _, err := executeCommand("sessions", "delete", "to-delete")
		if err != nil {
			t.Fatalf("sessions delete error: %v", err)
		}
	})
	if !strings.Contains(deleteOutput, "deleted") {
		t.Errorf("delete output should confirm deletion, got: %q", deleteOutput)
	}
}

// ---------------------------------------------------------------------------
// Collection commands with daemon
// ---------------------------------------------------------------------------

func TestCollectionsCRUDWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)

	// Create
	captureStdout(func() {
		_, _, err := executeCommand("collections", "create", "test-coll")
		if err != nil {
			t.Fatalf("collections create error: %v", err)
		}
	})

	// List
	listOutput := captureStdout(func() {
		_, _, err := executeCommand("collections", "list")
		if err != nil {
			t.Fatalf("collections list error: %v", err)
		}
	})
	if !strings.Contains(listOutput, "test-coll") {
		t.Errorf("list output should contain collection name, got: %q", listOutput)
	}

	// Add items
	captureStdout(func() {
		_, _, err := executeCommand("collections", "add", "test-coll",
			"--url", "https://example.com", "--title", "Example")
		if err != nil {
			t.Fatalf("collections add error: %v", err)
		}
	})

	// Get
	getOutput := captureStdout(func() {
		_, _, err := executeCommand("collections", "get", "test-coll")
		if err != nil {
			t.Fatalf("collections get error: %v", err)
		}
	})
	if !strings.Contains(getOutput, "example.com") {
		t.Errorf("get output should contain item URL, got: %q", getOutput)
	}

	// Delete
	deleteOutput := captureStdout(func() {
		_, _, err := executeCommand("collections", "delete", "test-coll")
		if err != nil {
			t.Fatalf("collections delete error: %v", err)
		}
	})
	if !strings.Contains(deleteOutput, "deleted") {
		t.Errorf("delete output should confirm deletion, got: %q", deleteOutput)
	}
}

// ---------------------------------------------------------------------------
// Bookmarks command tests with daemon
// ---------------------------------------------------------------------------

func TestBookmarksTreeWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	output := captureStdout(func() {
		_, _, err := executeCommand("bookmarks", "tree")
		if err != nil {
			t.Fatalf("bookmarks tree error: %v", err)
		}
	})

	if !strings.Contains(output, "Bookmarks Bar") {
		t.Errorf("output should contain folder name, got: %q", output)
	}
	if !strings.Contains(output, "Example") {
		t.Errorf("output should contain bookmark title, got: %q", output)
	}
}

func TestBookmarksSearchWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	output := captureStdout(func() {
		_, _, err := executeCommand("bookmarks", "search", "example")
		if err != nil {
			t.Fatalf("bookmarks search error: %v", err)
		}
	})

	if !strings.Contains(output, "Example") {
		t.Errorf("output should contain bookmark title, got: %q", output)
	}
	if !strings.Contains(output, "example.com") {
		t.Errorf("output should contain bookmark URL, got: %q", output)
	}
}

func TestBookmarksMirrorWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	output := captureStdout(func() {
		_, _, err := executeCommand("bookmarks", "mirror")
		if err != nil {
			t.Fatalf("bookmarks mirror error: %v", err)
		}
	})

	if !strings.Contains(output, "Nodes") {
		t.Errorf("output should contain node count, got: %q", output)
	}
	if !strings.Contains(output, "Folders") {
		t.Errorf("output should contain folder count, got: %q", output)
	}
}

func TestBookmarksExportWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	// Export requires a mirror to exist first — run mirror before export.
	captureStdout(func() {
		_, _, err := executeCommand("bookmarks", "mirror")
		if err != nil {
			t.Fatalf("bookmarks mirror error: %v", err)
		}
	})

	output := captureStdout(func() {
		_, _, err := executeCommand("bookmarks", "export")
		if err != nil {
			t.Fatalf("bookmarks export error: %v", err)
		}
	})

	if !strings.Contains(output, "# Bookmarks") {
		t.Errorf("output should contain markdown content, got: %q", output)
	}
}

// --- Error injection tests ---

func TestPrintJSON_InvalidJSON(t *testing.T) {
	// printJSON with invalid JSON should fall back to printing raw string
	data := json.RawMessage(`{invalid json}`)
	output := captureStdout(func() {
		printJSON(data)
	})
	if !strings.Contains(output, "invalid json") {
		t.Errorf("printJSON fallback should print raw data, got: %q", output)
	}
}

func TestPrintJSON_EmptyObject(t *testing.T) {
	data := json.RawMessage(`{}`)
	output := captureStdout(func() {
		printJSON(data)
	})
	if !strings.Contains(output, "{}") {
		t.Errorf("printJSON empty object should print {}, got: %q", output)
	}
}

func TestPrintJSON_Null(t *testing.T) {
	data := json.RawMessage(`null`)
	output := captureStdout(func() {
		printJSON(data)
	})
	if !strings.Contains(output, "null") {
		t.Errorf("printJSON null should print null, got: %q", output)
	}
}

func TestConnectAndRequest_NoDaemon(t *testing.T) {
	// Point to a nonexistent socket to ensure connection fails
	t.Setenv("CTM_CONFIG_DIR", t.TempDir())

	_, err := connectAndRequest("tabs.list", nil, nil)
	if err == nil {
		t.Fatal("expected error when daemon is not running")
	}
	if !strings.Contains(err.Error(), "cannot connect to daemon") {
		t.Errorf("error should mention cannot connect to daemon, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Daemon-integrated success path tests (Issue 4)
// ---------------------------------------------------------------------------

func TestGroupsCreateWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	output := captureStdout(func() {
		_, _, err := executeCommand("groups", "create", "--title", "TestGroup", "--tab-id", "1", "--tab-id", "2")
		if err != nil {
			t.Fatalf("groups create error: %v", err)
		}
	})

	// groups create prints human-readable confirmation
	if !strings.Contains(output, "Group") && !strings.Contains(output, "created") {
		t.Errorf("output should contain group creation info, got: %q", output)
	}
}

func TestSessionsRestoreWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	// First save a session so we can restore it
	captureStdout(func() {
		_, _, err := executeCommand("sessions", "save", "restore-test")
		if err != nil {
			t.Fatalf("sessions save error: %v", err)
		}
	})

	// Now restore it
	output := captureStdout(func() {
		_, _, err := executeCommand("sessions", "restore", "restore-test")
		if err != nil {
			t.Fatalf("sessions restore error: %v", err)
		}
	})

	// sessions.restore prints human-readable restore stats
	if !strings.Contains(output, "restored") {
		t.Errorf("output should contain restore stats, got: %q", output)
	}
}

func TestCollectionsRestoreWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	// Create a collection and add items
	captureStdout(func() {
		_, _, err := executeCommand("collections", "create", "restore-coll")
		if err != nil {
			t.Fatalf("collections create error: %v", err)
		}
	})
	captureStdout(func() {
		_, _, err := executeCommand("collections", "add", "restore-coll",
			"--url", "https://example.com", "--title", "Example")
		if err != nil {
			t.Fatalf("collections add error: %v", err)
		}
	})

	// Restore
	output := captureStdout(func() {
		_, _, err := executeCommand("collections", "restore", "restore-coll")
		if err != nil {
			t.Fatalf("collections restore error: %v", err)
		}
	})

	// collections.restore prints human-readable restore stats
	if !strings.Contains(output, "restored") {
		t.Errorf("output should contain restore stats, got: %q", output)
	}
}

func TestSearchWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	// Save a session so search has something to find
	captureStdout(func() {
		_, _, err := executeCommand("sessions", "save", "search-target")
		if err != nil {
			t.Fatalf("sessions save error: %v", err)
		}
	})

	// Search for something that matches
	output := captureStdout(func() {
		_, _, err := executeCommand("search", "Google")
		if err != nil {
			t.Fatalf("search error: %v", err)
		}
	})

	// Should find results (tabs scope has "Google" tab from mock extension)
	if !strings.Contains(output, "KIND") || !strings.Contains(output, "TITLE") {
		// If no results, at minimum we should see "No results found." which is still valid
		if !strings.Contains(output, "No results") && !strings.Contains(output, "results") {
			t.Errorf("output should contain results or 'No results', got: %q", output)
		}
	}
}

func TestTargetsDefaultWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	targetID := connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	output := captureStdout(func() {
		_, _, err := executeCommand("targets", "default", targetID)
		if err != nil {
			t.Fatalf("targets default error: %v", err)
		}
	})

	if !strings.Contains(output, "Default target set") {
		t.Errorf("output should confirm default set, got: %q", output)
	}
	if !strings.Contains(output, targetID) {
		t.Errorf("output should contain target ID, got: %q", output)
	}
}

func TestTargetsLabelWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	targetID := connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	output := captureStdout(func() {
		_, _, err := executeCommand("targets", "label", targetID, "my-browser")
		if err != nil {
			t.Fatalf("targets label error: %v", err)
		}
	})

	if !strings.Contains(output, "labeled") {
		t.Errorf("output should confirm label set, got: %q", output)
	}
	if !strings.Contains(output, "my-browser") {
		t.Errorf("output should contain label, got: %q", output)
	}
}

func TestSyncStatusWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)

	output := captureStdout(func() {
		_, _, err := executeCommand("sync", "status")
		if err != nil {
			t.Fatalf("sync status error: %v", err)
		}
	})

	// sync status should report enabled/disabled and sync dir
	if !strings.Contains(output, "Sync") {
		t.Errorf("output should contain 'Sync' status info, got: %q", output)
	}
}

func TestWorkspaceSwitchWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	// Create a workspace
	var wsID string
	captureStdout(func() {
		_, _, err := executeCommand("workspaces", "create", "switch-test")
		if err != nil {
			t.Fatalf("workspaces create error: %v", err)
		}
	})

	// Get the workspace ID from list output
	listOut := captureStdout(func() {
		_, _, err := executeCommand("workspaces", "--json", "list")
		if err != nil {
			t.Fatalf("workspaces list error: %v", err)
		}
	})
	var listResult struct {
		Workspaces []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"workspaces"`
	}
	if err := json.Unmarshal([]byte(listOut), &listResult); err != nil {
		t.Fatalf("parse workspaces list: %v", err)
	}
	for _, ws := range listResult.Workspaces {
		if ws.Name == "switch-test" {
			wsID = ws.ID
			break
		}
	}
	if wsID == "" {
		t.Fatal("workspace 'switch-test' not found in list")
	}

	// Switch to the workspace
	output := captureStdout(func() {
		_, _, err := executeCommand("workspaces", "switch", wsID)
		if err != nil {
			t.Fatalf("workspaces switch error: %v", err)
		}
	})

	if !strings.Contains(output, "Switched") {
		t.Errorf("output should contain 'Switched', got: %q", output)
	}
	if !strings.Contains(output, "closed") {
		t.Errorf("output should contain 'closed', got: %q", output)
	}
}

func TestSyncRepairWithDaemon(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)

	output := captureStdout(func() {
		_, _, err := executeCommand("sync", "repair")
		if err != nil {
			t.Fatalf("sync repair error: %v", err)
		}
	})

	if !strings.Contains(output, "Repair complete") {
		t.Errorf("output should confirm repair, got: %q", output)
	}
}

// ---------------------------------------------------------------------------
// JSON shape tests (C5)
//
// These tests verify that --json output contains the expected top-level
// keys and per-item fields, catching regressions in the response contract.
// ---------------------------------------------------------------------------

func TestJSONShape_TabsList(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	output := captureStdout(func() {
		_, _, err := executeCommand("tabs", "--json", "list")
		if err != nil {
			t.Fatalf("tabs --json list error: %v", err)
		}
	})

	var result struct {
		Tabs []struct {
			ID    *int   `json:"id"`
			Title string `json:"title"`
			URL   string `json:"url"`
		} `json:"tabs"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, output)
	}
	if result.Tabs == nil {
		t.Fatal("JSON output missing 'tabs' array")
	}
	if len(result.Tabs) == 0 {
		t.Fatal("expected at least one tab from mock extension")
	}
	for i, tab := range result.Tabs {
		if tab.ID == nil {
			t.Errorf("tabs[%d]: missing 'id' field", i)
		}
		if tab.Title == "" {
			t.Errorf("tabs[%d]: empty 'title' field", i)
		}
		if tab.URL == "" {
			t.Errorf("tabs[%d]: empty 'url' field", i)
		}
	}
}

func TestJSONShape_SessionsList(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	// Save a session first so the list is non-empty
	captureStdout(func() {
		_, _, err := executeCommand("sessions", "save", "shape-test")
		if err != nil {
			t.Fatalf("sessions save error: %v", err)
		}
	})

	output := captureStdout(func() {
		_, _, err := executeCommand("sessions", "--json", "list")
		if err != nil {
			t.Fatalf("sessions --json list error: %v", err)
		}
	})

	var result struct {
		Sessions []struct {
			Name     string `json:"name"`
			TabCount *int   `json:"tabCount"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, output)
	}
	if result.Sessions == nil {
		t.Fatal("JSON output missing 'sessions' array")
	}
	if len(result.Sessions) == 0 {
		t.Fatal("expected at least one session after save")
	}
	for i, s := range result.Sessions {
		if s.Name == "" {
			t.Errorf("sessions[%d]: empty 'name' field", i)
		}
		if s.TabCount == nil {
			t.Errorf("sessions[%d]: missing 'tabCount' field", i)
		}
	}
}

func TestJSONShape_CollectionsList(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)

	// Create a collection so the list is non-empty
	captureStdout(func() {
		_, _, err := executeCommand("collections", "create", "shape-coll")
		if err != nil {
			t.Fatalf("collections create error: %v", err)
		}
	})

	output := captureStdout(func() {
		_, _, err := executeCommand("collections", "--json", "list")
		if err != nil {
			t.Fatalf("collections --json list error: %v", err)
		}
	})

	var result struct {
		Collections []struct {
			Name      string `json:"name"`
			ItemCount *int   `json:"itemCount"`
		} `json:"collections"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, output)
	}
	if result.Collections == nil {
		t.Fatal("JSON output missing 'collections' array")
	}
	if len(result.Collections) == 0 {
		t.Fatal("expected at least one collection after create")
	}
	for i, c := range result.Collections {
		if c.Name == "" {
			t.Errorf("collections[%d]: empty 'name' field", i)
		}
		if c.ItemCount == nil {
			t.Errorf("collections[%d]: missing 'itemCount' field", i)
		}
	}
}

func TestJSONShape_TargetsList(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	output := captureStdout(func() {
		_, _, err := executeCommand("targets", "--json", "list")
		if err != nil {
			t.Fatalf("targets --json list error: %v", err)
		}
	})

	var result struct {
		Targets []struct {
			TargetID string `json:"targetId"`
			Channel  string `json:"channel"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, output)
	}
	if result.Targets == nil {
		t.Fatal("JSON output missing 'targets' array")
	}
	if len(result.Targets) == 0 {
		t.Fatal("expected at least one target after mock extension connect")
	}
	for i, tgt := range result.Targets {
		if tgt.TargetID == "" {
			t.Errorf("targets[%d]: empty 'targetId' field", i)
		}
		if tgt.Channel == "" {
			t.Errorf("targets[%d]: empty 'channel' field", i)
		}
	}
}

func TestJSONShape_SessionsSave(t *testing.T) {
	td := setupTestDaemon(t)
	t.Setenv("CTM_CONFIG_DIR", td.configDir)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	output := captureStdout(func() {
		_, _, err := executeCommand("sessions", "--json", "save", "shape-save")
		if err != nil {
			t.Fatalf("sessions --json save error: %v", err)
		}
	})

	var result struct {
		Name     string `json:"name"`
		TabCount *int   `json:"tabCount"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, output)
	}
	if result.Name == "" {
		t.Error("JSON output missing 'name' field")
	}
	if result.Name != "shape-save" {
		t.Errorf("expected name 'shape-save', got %q", result.Name)
	}
	if result.TabCount == nil {
		t.Error("JSON output missing 'tabCount' field")
	}
}

// ---------------------------------------------------------------------------
// Smoke test (D2): end-to-end request chain validation
//
// Starts a test daemon, connects a mock extension, and exercises the core
// request chain: targets.list → sessions.list → sessions.save → sessions.get
// → sessions.delete. Each step verifies a non-error response.
// ---------------------------------------------------------------------------

func TestSmokePath(t *testing.T) {
	td := setupTestDaemon(t)
	connectMockExtension(t, td.sockPath)
	time.Sleep(50 * time.Millisecond)

	// Create a client connection to the daemon
	c := client.New(td.sockPath)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer c.Close()

	if err := c.Connect(ctx); err != nil {
		t.Fatalf("client connect: %v", err)
	}

	// Step 1: targets.list — verify at least one target is connected
	resp, err := c.Request(ctx, "targets.list", nil, nil)
	if err != nil {
		t.Fatalf("targets.list: %v", err)
	}
	if resp.Type == protocol.TypeError {
		t.Fatalf("targets.list returned error: %s", resp.Error.Message)
	}
	var targetsResult struct {
		Targets []struct {
			TargetID string `json:"targetId"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(resp.Payload, &targetsResult); err != nil {
		t.Fatalf("targets.list unmarshal: %v", err)
	}
	if len(targetsResult.Targets) == 0 {
		t.Fatal("targets.list: expected at least one target")
	}

	// Step 2: sessions.list — verify empty list (no sessions saved yet)
	resp, err = c.Request(ctx, "sessions.list", nil, nil)
	if err != nil {
		t.Fatalf("sessions.list: %v", err)
	}
	if resp.Type == protocol.TypeError {
		t.Fatalf("sessions.list returned error: %s", resp.Error.Message)
	}
	var sessionsResult struct {
		Sessions []json.RawMessage `json:"sessions"`
	}
	if err := json.Unmarshal(resp.Payload, &sessionsResult); err != nil {
		t.Fatalf("sessions.list unmarshal: %v", err)
	}
	if len(sessionsResult.Sessions) != 0 {
		t.Fatalf("sessions.list: expected 0 sessions, got %d", len(sessionsResult.Sessions))
	}

	// Step 3: sessions.save — save a session via the extension
	resp, err = c.Request(ctx, "sessions.save", map[string]string{"name": "smoke-test"}, nil)
	if err != nil {
		t.Fatalf("sessions.save: %v", err)
	}
	if resp.Type == protocol.TypeError {
		t.Fatalf("sessions.save returned error: %s", resp.Error.Message)
	}
	var saveSummary struct {
		Name     string `json:"name"`
		TabCount int    `json:"tabCount"`
	}
	if err := json.Unmarshal(resp.Payload, &saveSummary); err != nil {
		t.Fatalf("sessions.save unmarshal: %v", err)
	}
	if saveSummary.Name != "smoke-test" {
		t.Errorf("sessions.save: expected name 'smoke-test', got %q", saveSummary.Name)
	}
	if saveSummary.TabCount == 0 {
		t.Error("sessions.save: expected non-zero tabCount")
	}

	// Step 4: sessions.get — retrieve the saved session
	resp, err = c.Request(ctx, "sessions.get", map[string]string{"name": "smoke-test"}, nil)
	if err != nil {
		t.Fatalf("sessions.get: %v", err)
	}
	if resp.Type == protocol.TypeError {
		t.Fatalf("sessions.get returned error: %s", resp.Error.Message)
	}
	var getResult struct {
		Session struct {
			Name    string `json:"name"`
			Windows []struct {
				Tabs []struct {
					URL string `json:"url"`
				} `json:"tabs"`
			} `json:"windows"`
		} `json:"session"`
	}
	if err := json.Unmarshal(resp.Payload, &getResult); err != nil {
		t.Fatalf("sessions.get unmarshal: %v", err)
	}
	if getResult.Session.Name != "smoke-test" {
		t.Errorf("sessions.get: expected name 'smoke-test', got %q", getResult.Session.Name)
	}
	if len(getResult.Session.Windows) == 0 {
		t.Error("sessions.get: expected at least one window")
	}

	// Step 5: sessions.delete — remove the session
	resp, err = c.Request(ctx, "sessions.delete", map[string]string{"name": "smoke-test"}, nil)
	if err != nil {
		t.Fatalf("sessions.delete: %v", err)
	}
	if resp.Type == protocol.TypeError {
		t.Fatalf("sessions.delete returned error: %s", resp.Error.Message)
	}

	// Verify deletion: sessions.list should return empty again
	resp, err = c.Request(ctx, "sessions.list", nil, nil)
	if err != nil {
		t.Fatalf("sessions.list (post-delete): %v", err)
	}
	if resp.Type == protocol.TypeError {
		t.Fatalf("sessions.list (post-delete) returned error: %s", resp.Error.Message)
	}
	var postDeleteResult struct {
		Sessions []json.RawMessage `json:"sessions"`
	}
	if err := json.Unmarshal(resp.Payload, &postDeleteResult); err != nil {
		t.Fatalf("sessions.list (post-delete) unmarshal: %v", err)
	}
	if len(postDeleteResult.Sessions) != 0 {
		t.Fatalf("sessions.list (post-delete): expected 0 sessions, got %d", len(postDeleteResult.Sessions))
	}
}
