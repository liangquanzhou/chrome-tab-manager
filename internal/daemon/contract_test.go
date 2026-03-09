package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// projectRoot returns the absolute path to the project root directory.
// The test runs from internal/daemon/, so we go up two levels.
func projectRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join(".", "..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve project root: %v", err)
	}
	return dir
}

// registryActionSet returns a map of all action names in ActionRegistry.
func registryActionSet() map[string]bool {
	m := make(map[string]bool, len(ActionRegistry))
	for _, meta := range ActionRegistry {
		m[meta.Action] = true
	}
	return m
}

// TestRegistryMatchesContracts verifies every action in ActionRegistry has
// a corresponding ## heading in doc/12_CONTRACTS.md.
func TestRegistryMatchesContracts(t *testing.T) {
	root := projectRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "doc", "12_CONTRACTS.md"))
	if err != nil {
		t.Fatalf("failed to read 12_CONTRACTS.md: %v", err)
	}

	// Extract action names from ## headings.
	// Matches lines like "## tabs.list", "## subscribe", "## bookmarks.overlay.set"
	// Action names may use camelCase (e.g. clearDefault, addItems, getText).
	// Action names always start with a lowercase letter (distinguishing them
	// from section headings like "## Conventions").
	re := regexp.MustCompile(`(?m)^## ([a-z][a-zA-Z0-9_.]+)$`)
	matches := re.FindAllStringSubmatch(string(data), -1)
	contractActions := make(map[string]bool, len(matches))
	for _, m := range matches {
		contractActions[m[1]] = true
	}

	if len(contractActions) == 0 {
		t.Fatal("found no action headings in 12_CONTRACTS.md — regex may be wrong")
	}

	// Every registry action must appear in contracts doc.
	for _, meta := range ActionRegistry {
		if !contractActions[meta.Action] {
			t.Errorf("action %q is in ActionRegistry but missing from 12_CONTRACTS.md", meta.Action)
		}
	}

	// Log contract-only actions (not a failure — contracts can document
	// protocol actions like "hello" that aren't in the registry).
	registry := registryActionSet()
	for action := range contractActions {
		if !registry[action] {
			t.Logf("note: %q is in 12_CONTRACTS.md but not in ActionRegistry (may be protocol-only)", action)
		}
	}
}

// TestRegistryMatchesMatrix verifies every action in ActionRegistry has a
// row in the capability matrix doc/19_CAPABILITY_MATRIX.md.
func TestRegistryMatchesMatrix(t *testing.T) {
	root := projectRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "doc", "19_CAPABILITY_MATRIX.md"))
	if err != nil {
		t.Fatalf("failed to read 19_CAPABILITY_MATRIX.md: %v", err)
	}

	// Extract action names from table rows that have a Level column (S/P/R).
	// This excludes event rows which have a different table structure.
	// Matches rows like: | tabs.list | V(fwd) | V | V | V(2) | V | S |
	levelRe := regexp.MustCompile(`(?m)^\| ([a-zA-Z][a-zA-Z0-9_.]+) \|.*\| ([SPR]) \|$`)
	levelMatches := levelRe.FindAllStringSubmatch(string(data), -1)
	matrixActions := make(map[string]bool, len(levelMatches))
	for _, m := range levelMatches {
		matrixActions[m[1]] = true
	}

	if len(matrixActions) == 0 {
		t.Fatal("found no action rows in 19_CAPABILITY_MATRIX.md — regex may be wrong")
	}

	// Every registry action must appear in matrix.
	for _, meta := range ActionRegistry {
		if !matrixActions[meta.Action] {
			t.Errorf("action %q is in ActionRegistry but missing from 19_CAPABILITY_MATRIX.md", meta.Action)
		}
	}

	// Every matrix action should be in registry.
	registry := registryActionSet()
	for action := range matrixActions {
		if !registry[action] {
			t.Errorf("action %q is in 19_CAPABILITY_MATRIX.md but missing from ActionRegistry", action)
		}
	}
}

// TestMatrixCountsConsistent parses the Support Level Summary section
// and verifies the stated S/P/R counts match the actual table rows.
func TestMatrixCountsConsistent(t *testing.T) {
	root := projectRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "doc", "19_CAPABILITY_MATRIX.md"))
	if err != nil {
		t.Fatalf("failed to read 19_CAPABILITY_MATRIX.md: %v", err)
	}
	content := string(data)

	// Count actual rows by level from the table.
	levelRe := regexp.MustCompile(`(?m)^\| [a-zA-Z][a-zA-Z0-9_.]+ \|.*\| ([SPR]) \|$`)
	levelMatches := levelRe.FindAllStringSubmatch(content, -1)

	actualCounts := map[string]int{"S": 0, "P": 0, "R": 0}
	for _, m := range levelMatches {
		actualCounts[m[1]]++
	}

	// Parse stated counts from the Summary section.
	// Looks for lines like "### S (Supported) -- 31 actions"
	summaryRe := regexp.MustCompile(`(?m)^### ([SPR]) \([^)]+\) -- (\d+) actions$`)
	summaryMatches := summaryRe.FindAllStringSubmatch(content, -1)
	if len(summaryMatches) != 3 {
		t.Fatalf("expected 3 summary entries (S/P/R), found %d", len(summaryMatches))
	}

	for _, m := range summaryMatches {
		level := m[1]
		stated := 0
		fmt.Sscanf(m[2], "%d", &stated)

		if stated != actualCounts[level] {
			t.Errorf("level %s: summary says %d actions but table has %d rows",
				level, stated, actualCounts[level])
		}
	}

	// Also verify total matches registry size.
	totalMatrix := actualCounts["S"] + actualCounts["P"] + actualCounts["R"]
	if totalMatrix != len(ActionRegistry) {
		t.Errorf("matrix total (%d) != ActionRegistry size (%d)",
			totalMatrix, len(ActionRegistry))
	}
}

// TestCLIExposureMatchesCode verifies that CLISupported actions have
// a connectAndRequest call in cmd/*.go, and CLIInternal actions do NOT.
func TestCLIExposureMatchesCode(t *testing.T) {
	root := projectRoot(t)
	cmdDir := filepath.Join(root, "cmd")

	// Read all .go files in cmd/ and concatenate their contents.
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		t.Fatalf("failed to read cmd/ directory: %v", err)
	}

	var allCode strings.Builder
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		// Skip test files — they may contain connectAndRequest calls for testing.
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(cmdDir, entry.Name()))
		if err != nil {
			t.Fatalf("failed to read cmd/%s: %v", entry.Name(), err)
		}
		allCode.Write(data)
		allCode.WriteByte('\n')
	}
	code := allCode.String()

	for _, meta := range ActionRegistry {
		// Search for connectAndRequest("action.name" in the combined code.
		needle := fmt.Sprintf(`connectAndRequest("%s"`, meta.Action)
		found := strings.Contains(code, needle)

		switch meta.CLI {
		case CLISupported:
			if !found {
				t.Errorf("action %q is CLISupported but no connectAndRequest(%q) found in cmd/*.go",
					meta.Action, meta.Action)
			}
		case CLIInternal:
			if found {
				t.Errorf("action %q is CLIInternal but connectAndRequest(%q) exists in cmd/*.go",
					meta.Action, meta.Action)
			}
		}
	}
}
