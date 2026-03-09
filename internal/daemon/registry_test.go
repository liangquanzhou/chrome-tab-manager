package daemon

import (
	"strings"
	"testing"
)

// TestRegistryNoDuplicates verifies no duplicate action names in the registry.
func TestRegistryNoDuplicates(t *testing.T) {
	seen := make(map[string]bool, len(ActionRegistry))
	for _, meta := range ActionRegistry {
		if meta.Action == "" {
			t.Fatal("registry entry with empty action name")
		}
		if seen[meta.Action] {
			t.Errorf("duplicate action in registry: %s", meta.Action)
		}
		seen[meta.Action] = true
	}
}

// TestRegistryCoversDispatcher verifies that every action explicitly handled
// by the hub dispatcher (switch statements in handle*Action methods) has a
// corresponding entry in ActionRegistry. Prefix-forwarded actions (tabs.*,
// groups.*, windows.*, history.*, downloads.*) are checked by prefix.
func TestRegistryCoversDispatcher(t *testing.T) {
	// Collect all actions from the registry
	registryActions := make(map[string]bool, len(ActionRegistry))
	for _, meta := range ActionRegistry {
		registryActions[meta.Action] = true
	}

	// Actions explicitly dispatched by handleRequest (non-prefix)
	explicitActions := []string{
		"daemon.stop",
		"subscribe",
	}

	// Actions dispatched inside handleTargetsAction
	targetsActions := []string{
		"targets.list",
		"targets.default",
		"targets.clearDefault",
		"targets.label",
	}

	// Actions dispatched inside handleSessionsAction
	sessionsActions := []string{
		"sessions.list",
		"sessions.get",
		"sessions.save",
		"sessions.restore",
		"sessions.delete",
	}

	// Actions dispatched inside handleCollectionsAction
	collectionsActions := []string{
		"collections.list",
		"collections.get",
		"collections.create",
		"collections.delete",
		"collections.addItems",
		"collections.removeItems",
		"collections.restore",
	}

	// Actions dispatched inside handleBookmarksAction
	bookmarksActions := []string{
		"bookmarks.tree",
		"bookmarks.search",
		"bookmarks.get",
		"bookmarks.mirror",
		"bookmarks.overlay.set",
		"bookmarks.overlay.get",
		"bookmarks.remove",
		"bookmarks.create",
		"bookmarks.update",
		"bookmarks.move",
		"bookmarks.export",
	}

	// Actions dispatched inside handleSearchAction
	searchActions := []string{
		"search.query",
		"search.saved.list",
		"search.saved.create",
		"search.saved.delete",
	}

	// Actions dispatched inside handleWorkspaceAction
	workspaceActions := []string{
		"workspace.list",
		"workspace.get",
		"workspace.create",
		"workspace.update",
		"workspace.delete",
		"workspace.switch",
	}

	// Actions dispatched inside handleSyncAction
	syncActions := []string{
		"sync.status",
		"sync.repair",
	}

	// Check that every explicitly dispatched action is in the registry
	var allExplicit []string
	allExplicit = append(allExplicit, explicitActions...)
	allExplicit = append(allExplicit, targetsActions...)
	allExplicit = append(allExplicit, sessionsActions...)
	allExplicit = append(allExplicit, collectionsActions...)
	allExplicit = append(allExplicit, bookmarksActions...)
	allExplicit = append(allExplicit, searchActions...)
	allExplicit = append(allExplicit, workspaceActions...)
	allExplicit = append(allExplicit, syncActions...)

	for _, action := range allExplicit {
		if !registryActions[action] {
			t.Errorf("action %q is dispatched by hub but missing from ActionRegistry", action)
		}
	}

	// Check that prefix-forwarded categories have at least one registry entry
	forwardedPrefixes := []string{"tabs.", "groups.", "windows.", "history.", "downloads."}
	for _, prefix := range forwardedPrefixes {
		found := false
		for _, meta := range ActionRegistry {
			if strings.HasPrefix(meta.Action, prefix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no registry entry for forwarded prefix %q", prefix)
		}
	}
}

// TestRegistryFieldsValid verifies that all registry entries use valid enum values.
func TestRegistryFieldsValid(t *testing.T) {
	validLayers := map[LayerType]bool{
		LayerForward: true,
		LayerLocal:   true,
		LayerHybrid:  true,
	}
	validTargets := map[TargetReq]bool{
		TargetRequired:   true,
		TargetOptional:   true,
		TargetDisallowed: true,
	}
	validCLI := map[CLIExposure]bool{
		CLISupported: true,
		CLIInternal:  true,
	}

	for _, meta := range ActionRegistry {
		if !validLayers[meta.Layer] {
			t.Errorf("action %q has invalid Layer: %q", meta.Action, meta.Layer)
		}
		if !validTargets[meta.Target] {
			t.Errorf("action %q has invalid Target: %q", meta.Action, meta.Target)
		}
		if !validCLI[meta.CLI] {
			t.Errorf("action %q has invalid CLI: %q", meta.Action, meta.CLI)
		}
	}
}
