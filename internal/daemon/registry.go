package daemon

// LayerType describes how the daemon processes an action.
type LayerType string

const (
	LayerForward LayerType = "forward" // Forward to extension via Native Messaging
	LayerLocal   LayerType = "local"   // Handled entirely within the daemon
	LayerHybrid  LayerType = "hybrid"  // Daemon + extension interaction
)

// TargetReq describes whether an action needs a connected browser target.
type TargetReq string

const (
	TargetRequired   TargetReq = "required"   // Must have a target
	TargetOptional   TargetReq = "optional"    // Works with or without a target
	TargetDisallowed TargetReq = "disallowed"  // No target needed
)

// CLIExposure describes how an action is exposed to the CLI.
type CLIExposure string

const (
	CLISupported CLIExposure = "supported" // Full CLI subcommand exists
	CLIInternal  CLIExposure = "internal"  // No CLI subcommand; used internally or by TUI
)

// ActionMeta describes a single action recognized by the hub dispatcher.
type ActionMeta struct {
	Action string // e.g. "tabs.list"
	Layer  LayerType
	Target TargetReq
	CLI    CLIExposure
}

// ActionRegistry is the canonical list of all actions handled by the daemon.
// CLI exposure is determined by whether cmd/*.go has a connectAndRequest call
// for this action. This table is the single source of truth for action metadata.
//
// Verified against: cmd/*.go (grep connectAndRequest), hub.go (dispatch),
// and 12_CONTRACTS.md on 2026-03-09.
var ActionRegistry = []ActionMeta{
	// --- Daemon control ---
	{"daemon.stop", LayerLocal, TargetDisallowed, CLIInternal},

	// --- Subscriptions ---
	{"subscribe", LayerLocal, TargetDisallowed, CLIInternal},

	// --- Targets (all have CLI in cmd/targets.go) ---
	{"targets.list", LayerLocal, TargetDisallowed, CLISupported},
	{"targets.default", LayerLocal, TargetDisallowed, CLISupported},
	{"targets.clearDefault", LayerLocal, TargetDisallowed, CLISupported},
	{"targets.label", LayerLocal, TargetDisallowed, CLISupported},

	// --- Sessions (all have CLI in cmd/sessions.go) ---
	{"sessions.list", LayerLocal, TargetDisallowed, CLISupported},
	{"sessions.get", LayerLocal, TargetDisallowed, CLISupported},
	{"sessions.save", LayerHybrid, TargetRequired, CLISupported},
	{"sessions.restore", LayerHybrid, TargetRequired, CLISupported},
	{"sessions.delete", LayerLocal, TargetDisallowed, CLISupported},

	// --- Collections (cmd/collections.go) ---
	{"collections.list", LayerLocal, TargetDisallowed, CLISupported},
	{"collections.get", LayerLocal, TargetDisallowed, CLISupported},
	{"collections.create", LayerLocal, TargetDisallowed, CLISupported},
	{"collections.delete", LayerLocal, TargetDisallowed, CLISupported},
	{"collections.addItems", LayerLocal, TargetDisallowed, CLISupported},
	{"collections.removeItems", LayerLocal, TargetDisallowed, CLISupported},
	{"collections.rename", LayerLocal, TargetDisallowed, CLIInternal},
	{"collections.reorder", LayerLocal, TargetDisallowed, CLIInternal},
	{"collections.restore", LayerHybrid, TargetRequired, CLISupported},

	// --- Bookmarks (cmd/bookmarks.go) ---
	{"bookmarks.tree", LayerHybrid, TargetRequired, CLISupported},
	{"bookmarks.search", LayerForward, TargetRequired, CLISupported},
	{"bookmarks.get", LayerForward, TargetRequired, CLIInternal},       // no CLI command
	{"bookmarks.mirror", LayerHybrid, TargetOptional, CLISupported},
	{"bookmarks.export", LayerLocal, TargetDisallowed, CLISupported},
	{"bookmarks.create", LayerForward, TargetRequired, CLISupported},
	{"bookmarks.update", LayerForward, TargetRequired, CLIInternal},    // no CLI command
	{"bookmarks.remove", LayerForward, TargetRequired, CLISupported},
	{"bookmarks.move", LayerForward, TargetRequired, CLIInternal},      // no CLI command
	{"bookmarks.overlay.set", LayerLocal, TargetDisallowed, CLIInternal}, // no CLI command
	{"bookmarks.overlay.get", LayerLocal, TargetDisallowed, CLIInternal}, // no CLI command

	// --- Search (cmd/search.go) ---
	{"search.query", LayerHybrid, TargetOptional, CLISupported},
	{"search.saved.list", LayerLocal, TargetDisallowed, CLISupported},
	{"search.saved.create", LayerLocal, TargetDisallowed, CLISupported},
	{"search.saved.delete", LayerLocal, TargetDisallowed, CLISupported},

	// --- Workspaces (cmd/workspace.go) ---
	{"workspace.list", LayerLocal, TargetDisallowed, CLISupported},
	{"workspace.get", LayerLocal, TargetDisallowed, CLISupported},
	{"workspace.create", LayerLocal, TargetDisallowed, CLISupported},
	{"workspace.update", LayerLocal, TargetDisallowed, CLISupported},
	{"workspace.delete", LayerLocal, TargetDisallowed, CLISupported},
	{"workspace.switch", LayerHybrid, TargetRequired, CLISupported},

	// --- Sync (cmd/sync.go) ---
	{"sync.status", LayerLocal, TargetDisallowed, CLISupported},
	{"sync.repair", LayerLocal, TargetDisallowed, CLISupported},

	// --- Browser runtime: Tabs (cmd/tabs.go) ---
	// Hub forwards all tabs.* to extension; only those with CLI commands are CLISupported.
	{"tabs.list", LayerForward, TargetRequired, CLISupported},
	{"tabs.open", LayerForward, TargetRequired, CLISupported},
	{"tabs.close", LayerForward, TargetRequired, CLISupported},
	{"tabs.activate", LayerForward, TargetRequired, CLISupported},
	{"tabs.update", LayerForward, TargetRequired, CLIInternal},     // no CLI; replaced by mute/pin
	{"tabs.mute", LayerForward, TargetRequired, CLISupported},
	{"tabs.pin", LayerForward, TargetRequired, CLISupported},
	{"tabs.move", LayerForward, TargetRequired, CLISupported},
	{"tabs.getText", LayerForward, TargetRequired, CLISupported},
	{"tabs.capture", LayerForward, TargetRequired, CLISupported},

	// --- Browser runtime: Groups (cmd/groups.go) ---
	{"groups.list", LayerForward, TargetRequired, CLISupported},
	{"groups.create", LayerForward, TargetRequired, CLISupported},
	{"groups.update", LayerForward, TargetRequired, CLISupported},
	{"groups.delete", LayerForward, TargetRequired, CLISupported},

	// --- Browser runtime: Windows (no CLI commands) ---
	{"windows.list", LayerForward, TargetRequired, CLIInternal},
	{"windows.create", LayerForward, TargetRequired, CLIInternal},
	{"windows.close", LayerForward, TargetRequired, CLIInternal},
	{"windows.focus", LayerForward, TargetRequired, CLIInternal},

	// --- Browser runtime: History (cmd/history.go) ---
	{"history.search", LayerForward, TargetRequired, CLISupported},
	{"history.delete", LayerForward, TargetRequired, CLISupported},

	// --- Browser runtime: Downloads (cmd/downloads.go) ---
	{"downloads.list", LayerForward, TargetRequired, CLISupported},
	{"downloads.cancel", LayerForward, TargetRequired, CLISupported},
}
