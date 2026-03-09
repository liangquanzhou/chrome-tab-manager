package tui

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"ctm/internal/client"
	"ctm/internal/protocol"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// --- Messages ---

type tickMsg time.Time
type toastClearMsg struct{}
type chordTimeoutMsg struct{}
type pendingGTimeoutMsg struct{}
type eventMsg *protocol.Message
type refreshMsg struct{ payload json.RawMessage }
type errMsg struct{ err error }
type toastMsg string
type targetRecoveredMsg struct{}
type previewTextMsg struct {
	tabID int
	text  string
}
type previewImageMsg struct {
	tabID   int
	dataURL string
}

// --- App Model ---

type App struct {
	client    *client.Client
	ctx       context.Context
	cancel    context.CancelFunc
	connected bool

	view      ViewType
	mode      InputMode
	views     map[ViewType]*ViewState
	width     int
	height    int

	// Event subscription
	eventCh <-chan *protocol.Message

	// Feedback
	toast       string
	errorMsg    string
	confirmHint string

	// Input buffers
	filterText  string
	commandText string
	nameText    string

	// Chord state
	pendingG bool

	// Target
	selectedTarget string

	// Bookmarks fold state
	collapsedFolders map[string]bool // folder ID -> collapsed
	bookmarkTree     []BookmarkItem  // original tree for re-flattening

	// Session/Collection expand state
	expandedSessions    map[string][]NestedTabItem
	expandedCollections map[string][]NestedTabItem

	// Tab preview
	previewMode         int            // 0=info, 1=text, 2=screenshot
	previewText         map[int]string // tabId -> extracted text
	previewImage        map[int]string // tabId -> base64 PNG data URL
	pendingPreviewFetch bool           // cursor moved, need to fetch preview

	// Search view state
	searchActive    bool   // true when showing search results (vs saved searches)
	lastSearchQuery string // last search query for saving

	// Name input prompt
	namePrompt string // prompt text for ModeNameInput
}

func NewApp(socketPath string) *App {
	ctx, cancel := context.WithCancel(context.Background())
	c := client.New(socketPath)

	views := make(map[ViewType]*ViewState)
	for _, v := range []ViewType{ViewTargets, ViewTabs, ViewGroups, ViewSessions, ViewCollections, ViewBookmarks, ViewWorkspaces, ViewSync, ViewHistory, ViewSearch, ViewDownloads} {
		views[v] = newViewState()
	}

	return &App{
		client:           c,
		ctx:              ctx,
		cancel:           cancel,
		view:             ViewTabs,
		mode:             ModeNormal,
		views:            views,
		width:            80,
		height:           24,
		collapsedFolders:    make(map[string]bool),
		expandedSessions:    make(map[string][]NestedTabItem),
		expandedCollections: make(map[string][]NestedTabItem),
		previewText:      make(map[int]string),
		previewImage:     make(map[int]string),
	}
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.connectCmd(),
		tea.WindowSize(),
	)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case tea.KeyMsg:
		model, cmd := a.handleKey(msg)
		// After key handling, trigger preview fetch if cursor moved in preview mode
		if a.pendingPreviewFetch {
			a.pendingPreviewFetch = false
			previewCmd := a.fetchTabPreview()
			if previewCmd != nil {
				cmd = tea.Batch(cmd, previewCmd)
			}
		}
		return model, cmd

	case tea.MouseMsg:
		return a.handleMouse(msg)

	case refreshMsg:
		if msg.payload != nil {
			a.applyRefresh(msg.payload)
			return a, a.waitForEvent()
		}
		// nil payload = action completed without refresh data (delete, subscribe, etc.)
		// trigger an explicit re-fetch so the list updates immediately
		return a, tea.Batch(a.refreshCurrentView(), a.waitForEvent())

	case eventMsg:
		return a, tea.Batch(a.refreshCurrentView(), a.waitForEvent())

	case errMsg:
		errStr := msg.err.Error()
		// Auto-recover from stale target: clear selectedTarget, re-fetch targets, retry
		if strings.Contains(errStr, "TARGET_OFFLINE") && a.selectedTarget != "" {
			a.selectedTarget = ""
			a.errorMsg = ""
			return a, a.autoSelectTargetAndRetry()
		}
		a.errorMsg = errStr
		return a, nil

	case previewTextMsg:
		a.previewText[msg.tabID] = msg.text
		return a, nil

	case previewImageMsg:
		a.previewImage[msg.tabID] = msg.dataURL
		return a, nil

	case toastMsg:
		a.toast = string(msg)
		return a, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return toastClearMsg{} })

	case targetRecoveredMsg:
		a.toast = fmt.Sprintf("target recovered: %s", a.selectedTarget)
		return a, tea.Batch(a.refreshCurrentView(), tea.Tick(3*time.Second, func(time.Time) tea.Msg { return toastClearMsg{} }))

	case toastClearMsg:
		a.toast = ""
		return a, nil

	case chordTimeoutMsg:
		if a.mode == ModeYank || a.mode == ModeZFilter || a.mode == ModeConfirmDelete {
			a.mode = ModeNormal
			a.confirmHint = ""
		}
		return a, nil

	case pendingGTimeoutMsg:
		a.pendingG = false
		return a, nil
	}

	return a, nil
}

func (a *App) View() string {
	if a.mode == ModeHelp {
		return a.renderHelp()
	}

	header := a.renderHeader()
	statusBar := a.renderStatusBar()
	filterBar := ""
	if a.mode == ModeFilter {
		filterBar = styleFilter.Render("/ "+a.filterText+"_") + "\n"
	}
	if a.mode == ModeCommand {
		filterBar = styleFilter.Render(": "+a.commandText+"_") + "\n"
	}
	if a.mode == ModeNameInput {
		prompt := a.namePrompt
		if prompt == "" {
			prompt = "Name: "
		}
		filterBar = styleFilter.Render(prompt+a.nameText+"_") + "\n"
	}

	contentHeight := a.height - 3 // header(2) + status(1)
	if filterBar != "" {
		contentHeight--
	}
	if contentHeight < 1 {
		contentHeight = 1
	}

	content := a.renderContent(contentHeight)

	return header + "\n" + filterBar + content + "\n" + statusBar
}

// --- Key handling ---

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Mode-specific handling first
	switch a.mode {
	case ModeFilter:
		return a.handleFilterKey(key, msg)
	case ModeCommand:
		return a.handleCommandKey(key, msg)
	case ModeNameInput:
		return a.handleNameInputKey(key, msg)
	case ModeHelp:
		if key == "esc" || key == "q" || key == "?" {
			a.mode = ModeNormal
		}
		return a, nil
	case ModeYank:
		return a.handleYankKey(key)
	case ModeZFilter:
		return a.handleZFilterKey(key)
	case ModeConfirmDelete:
		return a.handleConfirmDeleteKey(key)
	}

	// Normal mode — pendingG check
	if a.pendingG && key == "g" {
		a.pendingG = false
		a.currentView().cursor = 0
		if a.view == ViewTabs && a.previewMode > 0 {
			a.pendingPreviewFetch = true
		}
		return a, nil
	}
	a.pendingG = false

	// Normal mode keys
	switch key {
	case "q":
		a.cancel()
		return a, tea.Quit
	case "esc":
		a.errorMsg = ""
		a.confirmHint = ""
		if a.view == ViewSearch && a.searchActive {
			a.searchActive = false
			return a, a.refreshCurrentView()
		}
		return a, nil
	case "?":
		a.mode = ModeHelp
		return a, nil
	case "/":
		a.mode = ModeFilter
		a.filterText = ""
		return a, nil
	case ":":
		a.mode = ModeCommand
		a.commandText = ""
		return a, nil
	case "r":
		return a, a.refreshCurrentView()
	case "tab":
		a.nextView(1)
		return a, a.refreshCurrentView()
	case "shift+tab":
		a.nextView(-1)
		return a, a.refreshCurrentView()
	case "1":
		a.view = ViewTargets
		return a, a.refreshCurrentView()
	case "2":
		a.view = ViewTabs
		return a, a.refreshCurrentView()
	case "3":
		a.view = ViewGroups
		return a, a.refreshCurrentView()
	case "4":
		a.view = ViewSessions
		return a, a.refreshCurrentView()
	case "5":
		a.view = ViewCollections
		return a, a.refreshCurrentView()
	case "6":
		a.view = ViewBookmarks
		return a, a.refreshCurrentView()
	case "7":
		a.view = ViewWorkspaces
		return a, a.refreshCurrentView()
	case "8":
		a.view = ViewSync
		return a, a.refreshCurrentView()
	case "9":
		a.view = ViewHistory
		return a, a.refreshCurrentView()
	case "0":
		a.view = ViewSearch
		return a, a.refreshCurrentView()

	// Navigation
	case "j", "down":
		a.moveCursor(1)
	case "k", "up":
		a.moveCursor(-1)
	case "g":
		a.pendingG = true
		return a, tea.Tick(200*time.Millisecond, func(time.Time) tea.Msg { return pendingGTimeoutMsg{} })
	case "G", "end":
		vs := a.currentView()
		old := vs.cursor
		vs.cursor = vs.visibleCount() - 1
		if vs.cursor < 0 {
			vs.cursor = 0
		}
		if a.view == ViewTabs && vs.cursor != old && a.previewMode > 0 {
			a.pendingPreviewFetch = true
		}
	case "ctrl+d":
		a.moveCursor(a.contentHeight() / 2)
	case "ctrl+u":
		a.moveCursor(-a.contentHeight() / 2)
	case " ":
		a.toggleSelect()
	case "enter":
		return a, a.handleEnter()
	case "ctrl+a":
		a.selectAll()
	case "u":
		a.clearSelection()

	// View-specific
	case "x":
		if a.view == ViewTabs {
			return a, a.closeTabs()
		}
		if a.view == ViewDownloads {
			return a, a.cancelDownload()
		}
		if a.view == ViewCollections {
			vs := a.currentView()
			idx := vs.realIndex(vs.cursor)
			if idx < len(vs.items) {
				if nested, ok := vs.items[idx].(NestedTabItem); ok {
					return a, a.doRequest("collections.removeItems", map[string]any{
						"name": nested.ParentName,
						"urls": []string{nested.URL},
					}, func(_ json.RawMessage) {
						a.toast = fmt.Sprintf("Removed %q from %q", truncate(nested.Title, 20), nested.ParentName)
					})
				}
			}
		}
	case "n":
		if a.view == ViewSessions || a.view == ViewCollections || a.view == ViewWorkspaces {
			a.mode = ModeNameInput
			a.nameText = ""
			a.namePrompt = "Name: "
			return a, nil
		}
		if a.view == ViewTabs {
			a.mode = ModeNameInput
			a.nameText = ""
			a.namePrompt = "Group name: "
			return a, nil
		}
		if a.view == ViewSearch && a.searchActive && a.lastSearchQuery != "" {
			a.mode = ModeNameInput
			a.nameText = ""
			a.namePrompt = "Save as: "
			return a, nil
		}
	case "o":
		if a.view == ViewWorkspaces {
			return a, a.switchWorkspace()
		}
		return a, a.handleRestore()
	case "D":
		if a.view == ViewSessions || a.view == ViewCollections || a.view == ViewWorkspaces {
			a.mode = ModeConfirmDelete
			name := a.currentItemName()
			a.confirmHint = fmt.Sprintf("Press D again to delete %q", name)
			return a, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return chordTimeoutMsg{} })
		}
		if a.view == ViewGroups {
			vs := a.currentView()
			idx := vs.realIndex(vs.cursor)
			if idx < len(vs.items) {
				g := vs.items[idx].(GroupItem)
				a.mode = ModeConfirmDelete
				a.confirmHint = fmt.Sprintf("Press D again to delete group %q", g.Title)
				return a, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return chordTimeoutMsg{} })
			}
		}
		if a.view == ViewBookmarks {
			vs := a.currentView()
			idx := vs.realIndex(vs.cursor)
			if idx < len(vs.items) {
				bm := vs.items[idx].(BookmarkItem)
				// Only Chrome's invisible root (id "0") is truly undeletable
				if bm.ID == "0" {
					a.errorMsg = "Chrome root node cannot be deleted"
					return a, nil
				}
				a.mode = ModeConfirmDelete
				label := bm.Title
				if bm.IsFolder {
					label += " (folder)"
				}
				a.confirmHint = fmt.Sprintf("Press D again to delete %q from Chrome", label)
				return a, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return chordTimeoutMsg{} })
			}
		}
		if a.view == ViewHistory {
			vs := a.currentView()
			idx := vs.realIndex(vs.cursor)
			if idx < len(vs.items) {
				h := vs.items[idx].(HistoryItem)
				a.mode = ModeConfirmDelete
				a.confirmHint = fmt.Sprintf("Press D again to delete %q from history", h.Title)
				return a, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return chordTimeoutMsg{} })
			}
		}
		if a.view == ViewSearch && !a.searchActive {
			vs := a.currentView()
			idx := vs.realIndex(vs.cursor)
			if idx < len(vs.items) {
				if saved, ok := vs.items[idx].(SavedSearchItem); ok {
					a.mode = ModeConfirmDelete
					a.confirmHint = fmt.Sprintf("Press D again to delete saved search %q", saved.Name)
					return a, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return chordTimeoutMsg{} })
				}
			}
		}
	case "y":
		a.mode = ModeYank
		a.confirmHint = "y:URL n:name h:host m:markdown"
		return a, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return chordTimeoutMsg{} })
	case "z":
		if a.view == ViewTabs {
			a.mode = ModeZFilter
			a.confirmHint = "h:host p:pinned g:grouped a:active c:clear"
			return a, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return chordTimeoutMsg{} })
		}
		if a.view == ViewBookmarks {
			a.mode = ModeZFilter
			a.confirmHint = "M:fold all  R:unfold all"
			return a, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return chordTimeoutMsg{} })
		}
	case "v":
		if a.view == ViewTabs {
			a.previewMode = (a.previewMode + 1) % 2 // 0=info, 1=text
			modes := []string{"info", "text"}
			a.toast = "Preview: " + modes[a.previewMode]
			// Clear cached preview for current tab to force fresh fetch
			if tab, ok := a.currentTab(); ok {
				delete(a.previewText, tab.ID)
			}
			if a.previewMode == 1 {
				return a, tea.Batch(a.fetchTabPreview(), tea.Tick(3*time.Second, func(time.Time) tea.Msg { return toastClearMsg{} }))
			}
			return a, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return toastClearMsg{} })
		}
	case "s":
		if a.view == ViewTabs {
			if a.previewMode == 2 {
				a.previewMode = 0
			} else {
				a.previewMode = 2
			}
			modes := []string{"info", "text", "screenshot"}
			a.toast = "Preview: " + modes[a.previewMode]
			if a.previewMode == 2 {
				return a, tea.Batch(a.fetchTabPreview(), tea.Tick(3*time.Second, func(time.Time) tea.Msg { return toastClearMsg{} }))
			}
			return a, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return toastClearMsg{} })
		}
	case "M":
		if a.view == ViewTabs {
			a.mode = ModeNameInput
			a.nameText = ""
			a.namePrompt = "Move to window: "
			return a, nil
		}
	case "A":
		if a.view == ViewTabs {
			a.mode = ModeNameInput
			a.nameText = ""
			a.namePrompt = "Add to collection: "
			return a, nil
		}
	case "R":
		if a.view == ViewSync {
			return a, a.doRequest("sync.repair", nil, func(_ json.RawMessage) {
				a.toast = "Sync repair completed"
			})
		}
	case "e":
		if a.view == ViewWorkspaces {
			a.mode = ModeNameInput
			a.nameText = ""
			a.namePrompt = "New name: "
			return a, nil
		}
	case "P":
		if a.view == ViewTabs {
			return a, a.openW3M()
		}
	case "m":
		if a.view == ViewTabs {
			return a, a.toggleTabMute()
		}
	case "p":
		if a.view == ViewTabs {
			return a, a.toggleTabPin()
		}
	case "c":
		if a.view == ViewTargets {
			return a, a.doRequest("targets.clearDefault", nil, func(_ json.RawMessage) {
				a.selectedTarget = ""
				a.toast = "Default target cleared"
			})
		}
	case "d":
		if a.view == ViewTargets {
			return a, a.setDefaultTarget()
		}
	case "a":
		if a.view == ViewBookmarks {
			a.mode = ModeNameInput
			a.nameText = ""
			a.namePrompt = "URL: "
			return a, nil
		}
	case "E":
		if a.view == ViewBookmarks {
			return a, a.doRequest("bookmarks.export", nil, func(payload json.RawMessage) {
				var result struct {
					Content string `json:"content"`
				}
				json.Unmarshal(payload, &result)
				if result.Content != "" {
					clipboard.WriteAll(result.Content)
					a.toast = "Bookmarks exported to clipboard"
				} else {
					a.toast = "No bookmarks to export (run mirror first)"
				}
			})
		}
	case "l", "right":
		// Expand folder in bookmarks view
		if a.view == ViewBookmarks {
			vs := a.currentView()
			if vs.visibleCount() > 0 {
				idx := vs.realIndex(vs.cursor)
				if idx < len(vs.items) {
					bm := vs.items[idx].(BookmarkItem)
					if bm.IsFolder && a.collapsedFolders[bm.ID] {
						delete(a.collapsedFolders, bm.ID)
						a.reflattenBookmarks()
					}
				}
			}
		}
		if key == "l" && a.view == ViewTargets {
			vs := a.currentView()
			if vs.cursor < len(vs.items) {
				a.mode = ModeNameInput
				a.nameText = ""
				a.namePrompt = "Label: "
				return a, nil
			}
		}
	case "h", "left":
		// Collapse folder in bookmarks view (or go to parent)
		if a.view == ViewBookmarks {
			vs := a.currentView()
			if vs.visibleCount() > 0 {
				idx := vs.realIndex(vs.cursor)
				if idx < len(vs.items) {
					bm := vs.items[idx].(BookmarkItem)
					if bm.IsFolder && !a.collapsedFolders[bm.ID] && len(bm.Children) > 0 {
						// Collapse this folder
						a.collapsedFolders[bm.ID] = true
						a.reflattenBookmarks()
					} else if bm.Depth > 0 {
						// Go to parent folder
						for i := idx - 1; i >= 0; i-- {
							parent := vs.items[i].(BookmarkItem)
							if parent.IsFolder && parent.Depth < bm.Depth {
								vs.cursor = i
								break
							}
						}
					}
				}
			}
		}
	}

	return a, nil
}

func (a *App) handleFilterKey(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key {
	case "enter":
		a.mode = ModeNormal
		if a.view == ViewSearch && a.filterText != "" {
			query := a.filterText
			a.filterText = ""
			return a, a.performSearch(query)
		}
		a.applyFilter()
	case "esc":
		a.mode = ModeNormal
		a.filterText = ""
		a.clearFilter()
	case "backspace":
		runes := []rune(a.filterText)
		if len(runes) > 0 {
			a.filterText = string(runes[:len(runes)-1])
		}
		a.applyFilter()
	default:
		if msg.Type == tea.KeyRunes {
			a.filterText += string(msg.Runes)
			a.applyFilter()
		}
	}
	return a, nil
}

func (a *App) handleCommandKey(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key {
	case "enter":
		cmd := a.executeCommand(a.commandText)
		a.commandText = ""
		// Only reset to Normal if executeCommand didn't change mode
		// (e.g. :help sets ModeHelp, which should not be overridden).
		if a.mode == ModeCommand {
			a.mode = ModeNormal
		}
		return a, cmd
	case "esc":
		a.mode = ModeNormal
		a.commandText = ""
	case "backspace":
		runes := []rune(a.commandText)
		if len(runes) > 0 {
			a.commandText = string(runes[:len(runes)-1])
		}
	default:
		if msg.Type == tea.KeyRunes {
			a.commandText += string(msg.Runes)
		}
	}
	return a, nil
}

func (a *App) handleNameInputKey(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key {
	case "enter":
		name := a.nameText
		a.mode = ModeNormal
		a.nameText = ""
		if name == "" {
			return a, nil
		}
		if a.view == ViewSessions {
			return a, a.saveSession(name)
		}
		if a.view == ViewCollections {
			return a, a.createCollection(name)
		}
		if a.view == ViewWorkspaces {
			if a.namePrompt == "New name: " {
				// workspace.update (rename)
				vs := a.currentView()
				idx := vs.realIndex(vs.cursor)
				if idx < len(vs.items) {
					ws := vs.items[idx].(WorkspaceItem)
					return a, a.doRequest("workspace.update", map[string]any{"id": ws.ID, "name": name}, func(_ json.RawMessage) {
						a.toast = fmt.Sprintf("Renamed workspace to %q", name)
					})
				}
				return a, nil
			}
			return a, a.doRequest("workspace.create", map[string]any{"name": name}, func(_ json.RawMessage) {
				a.toast = fmt.Sprintf("Workspace %q created", name)
			})
		}
		if a.view == ViewTabs && a.namePrompt == "Group name: " {
			// groups.create: collect selected tabs (or current tab) as tabIds
			vs := a.views[ViewTabs]
			var tabIDs []int
			if len(vs.selected) > 0 {
				for idx := range vs.selected {
					if idx < len(vs.items) {
						tab := vs.items[idx].(TabItem)
						tabIDs = append(tabIDs, tab.ID)
					}
				}
				vs.selected = make(map[int]bool)
			} else {
				ri := vs.realIndex(vs.cursor)
				if ri < len(vs.items) {
					tab := vs.items[ri].(TabItem)
					tabIDs = append(tabIDs, tab.ID)
				}
			}
			if len(tabIDs) == 0 {
				a.errorMsg = "No tabs to group"
				return a, nil
			}
			return a, a.doRequest("groups.create", map[string]any{
				"tabIds": tabIDs,
				"title":  name,
			}, func(_ json.RawMessage) {
				a.toast = fmt.Sprintf("Created group %q with %d tab(s)", name, len(tabIDs))
			})
		}
		if a.view == ViewTabs && a.namePrompt == "Move to window: " {
			windowID, err := strconv.Atoi(name)
			if err != nil {
				a.errorMsg = "Invalid window ID"
				return a, nil
			}
			vs := a.views[ViewTabs]
			idx := vs.realIndex(vs.cursor)
			if idx < len(vs.items) {
				tab := vs.items[idx].(TabItem)
				return a, a.doRequest("tabs.move", map[string]any{"tabId": tab.ID, "windowId": windowID}, func(_ json.RawMessage) {
					a.toast = fmt.Sprintf("Moved tab to window %d", windowID)
				})
			}
			return a, nil
		}
		if a.view == ViewTabs && a.namePrompt == "Add to collection: " {
			vs := a.views[ViewTabs]
			var items []map[string]string
			if len(vs.selected) > 0 {
				for idx := range vs.selected {
					if idx < len(vs.items) {
						tab := vs.items[idx].(TabItem)
						items = append(items, map[string]string{"url": tab.URL, "title": tab.Title})
					}
				}
				vs.selected = make(map[int]bool)
			} else {
				idx := vs.realIndex(vs.cursor)
				if idx < len(vs.items) {
					tab := vs.items[idx].(TabItem)
					items = append(items, map[string]string{"url": tab.URL, "title": tab.Title})
				}
			}
			if len(items) == 0 {
				return a, nil
			}
			itemCount := len(items)
			return a, a.doRequest("collections.addItems", map[string]any{"name": name, "items": items}, func(_ json.RawMessage) {
				a.toast = fmt.Sprintf("Added %d tab(s) to %q", itemCount, name)
			})
		}
		if a.view == ViewTargets {
			vs := a.currentView()
			idx := vs.realIndex(vs.cursor)
			if idx < len(vs.items) {
				t := vs.items[idx].(TargetItem)
				return a, a.doRequest("targets.label", map[string]string{
					"targetId": t.TargetID, "label": name,
				}, func(_ json.RawMessage) {
					a.toast = fmt.Sprintf("Target labeled %q", name)
				})
			}
		}
		if a.view == ViewBookmarks {
			// name is actually a URL entered via 'a' key
			return a, a.doRequest("bookmarks.create", map[string]any{
				"url": name,
			}, func(payload json.RawMessage) {
				var result struct {
					Bookmark struct {
						Title string `json:"title"`
					} `json:"bookmark"`
				}
				json.Unmarshal(payload, &result)
				title := result.Bookmark.Title
				if title == "" {
					title = name
				}
				a.toast = fmt.Sprintf("Bookmark created: %s", title)
			})
		}
		if a.view == ViewSearch {
			// Save current search query
			return a, a.doRequest("search.saved.create", map[string]any{
				"name":  name,
				"query": map[string]any{"query": a.lastSearchQuery},
			}, func(_ json.RawMessage) {
				a.toast = fmt.Sprintf("Search saved as %q", name)
			})
		}
	case "esc":
		a.mode = ModeNormal
		a.nameText = ""
	case "backspace":
		runes := []rune(a.nameText)
		if len(runes) > 0 {
			a.nameText = string(runes[:len(runes)-1])
		}
	default:
		if msg.Type == tea.KeyRunes {
			a.nameText += string(msg.Runes)
		}
	}
	return a, nil
}

func (a *App) handleYankKey(key string) (tea.Model, tea.Cmd) {
	a.mode = ModeNormal
	a.confirmHint = ""

	vs := a.currentView()
	if vs.visibleCount() == 0 {
		return a, nil
	}

	idx := vs.realIndex(vs.cursor)
	var text string

	switch a.view {
	case ViewTabs:
		if idx < len(vs.items) {
			tab := vs.items[idx].(TabItem)
			switch key {
			case "y":
				text = tab.URL
			case "n":
				text = tab.Title
			case "h":
				text = extractHost(tab.URL)
			case "m":
				text = fmt.Sprintf("[%s](%s)", tab.Title, tab.URL)
			default:
				return a, nil
			}
		}
	default:
		return a, nil
	}

	if text != "" {
		clipboard.WriteAll(text)
		return a, a.showToast("Copied!")
	}
	return a, nil
}

func (a *App) handleZFilterKey(key string) (tea.Model, tea.Cmd) {
	a.mode = ModeNormal
	a.confirmHint = ""

	// Bookmarks fold/unfold: zM / zR
	if a.view == ViewBookmarks {
		switch key {
		case "M":
			// Fold all folders
			for _, item := range a.bookmarkTree {
				a.foldAllBookmarks(item)
			}
			a.reflattenBookmarks()
			a.toast = "All folders folded"
			return a, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return toastClearMsg{} })
		case "R":
			// Unfold all folders
			a.collapsedFolders = make(map[string]bool)
			a.reflattenBookmarks()
			a.toast = "All folders unfolded"
			return a, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return toastClearMsg{} })
		}
		return a, nil
	}

	vs := a.views[ViewTabs]
	if len(vs.items) == 0 {
		return a, nil
	}

	switch key {
	case "h":
		// Filter by host of current tab
		idx := vs.realIndex(vs.cursor)
		if idx >= len(vs.items) {
			return a, nil
		}
		tab := vs.items[idx].(TabItem)
		host := extractHost(tab.URL)
		vs.filtered = nil
		for i, item := range vs.items {
			if extractHost(item.(TabItem).URL) == host {
				vs.filtered = append(vs.filtered, i)
			}
		}
		vs.clampCursor()
		a.toast = fmt.Sprintf("Filtered: %s", host)
		return a, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return toastClearMsg{} })
	case "p":
		vs.filtered = nil
		for i, item := range vs.items {
			if item.(TabItem).Pinned {
				vs.filtered = append(vs.filtered, i)
			}
		}
		vs.clampCursor()
		a.toast = "Filtered: pinned"
		return a, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return toastClearMsg{} })
	case "g":
		vs.filtered = nil
		for i, item := range vs.items {
			if item.(TabItem).GroupID >= 0 {
				vs.filtered = append(vs.filtered, i)
			}
		}
		vs.clampCursor()
		a.toast = "Filtered: grouped"
		return a, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return toastClearMsg{} })
	case "a":
		vs.filtered = nil
		for i, item := range vs.items {
			if item.(TabItem).Active {
				vs.filtered = append(vs.filtered, i)
			}
		}
		vs.clampCursor()
		a.toast = "Filtered: active"
		return a, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return toastClearMsg{} })
	case "c":
		vs.filtered = nil
		vs.clampCursor()
		a.toast = "Filter cleared"
		return a, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return toastClearMsg{} })
	}

	return a, nil
}

func (a *App) handleConfirmDeleteKey(key string) (tea.Model, tea.Cmd) {
	a.confirmHint = ""
	if key != "D" {
		a.mode = ModeNormal
		return a, nil
	}
	a.mode = ModeNormal

	// Views that delete by item ID/URL (not by name)

	// History: delete from browser history
	if a.view == ViewHistory {
		vs := a.currentView()
		idx := vs.realIndex(vs.cursor)
		if idx >= len(vs.items) {
			return a, nil
		}
		h := vs.items[idx].(HistoryItem)
		return a, a.doRequest("history.delete", map[string]string{"url": h.URL}, func(_ json.RawMessage) {
			a.toast = fmt.Sprintf("Deleted %q from history", h.Title)
		})
	}

	// Search: delete saved search
	if a.view == ViewSearch {
		vs := a.currentView()
		idx := vs.realIndex(vs.cursor)
		if idx < len(vs.items) {
			if saved, ok := vs.items[idx].(SavedSearchItem); ok {
				return a, a.doRequest("search.saved.delete", map[string]string{"id": saved.ID}, func(_ json.RawMessage) {
					a.toast = fmt.Sprintf("Deleted saved search %q", saved.Name)
				})
			}
		}
		return a, nil
	}

	// Bookmarks: delete from Chrome via extension, then re-fetch tree
	if a.view == ViewBookmarks {
		vs := a.currentView()
		idx := vs.realIndex(vs.cursor)
		if idx >= len(vs.items) {
			return a, nil
		}
		bm := vs.items[idx].(BookmarkItem)
		if bm.ID == "0" {
			a.errorMsg = "Chrome root node cannot be deleted"
			return a, nil
		}
		target := a.targetSelector()
		bmID, bmTitle := bm.ID, bm.Title
		return a, func() tea.Msg {
			_, err := a.client.Request(a.ctx, "bookmarks.remove", map[string]string{"id": bmID}, target)
			if err != nil {
				return errMsg{err}
			}
			a.toast = fmt.Sprintf("Deleted %q from Chrome", bmTitle)
			// Re-fetch tree immediately for instant feedback
			resp, err := a.client.Request(a.ctx, "bookmarks.tree", nil, target)
			if err != nil {
				return toastMsg(fmt.Sprintf("Deleted %q (refresh failed)", bmTitle))
			}
			return refreshMsg{payload: resp.Payload}
		}
	}

	// Groups: delete by groupId
	if a.view == ViewGroups {
		vs := a.currentView()
		idx := vs.realIndex(vs.cursor)
		if idx < len(vs.items) {
			g := vs.items[idx].(GroupItem)
			return a, a.doRequest("groups.delete", map[string]any{"groupId": g.ID}, func(_ json.RawMessage) {
				a.toast = fmt.Sprintf("Deleted group %q", g.Title)
			})
		}
		return a, nil
	}

	// Views that delete by name
	name := a.currentItemName()
	if name == "" {
		return a, nil
	}

	switch a.view {
	case ViewSessions:
		return a, a.doRequest("sessions.delete", map[string]string{"name": name}, func(_ json.RawMessage) {
			a.toast = fmt.Sprintf("Deleted %q", name)
		})
	case ViewCollections:
		return a, a.doRequest("collections.delete", map[string]string{"name": name}, func(_ json.RawMessage) {
			a.toast = fmt.Sprintf("Deleted %q", name)
		})
	case ViewWorkspaces:
		vs := a.currentView()
		idx := vs.realIndex(vs.cursor)
		if idx < len(vs.items) {
			ws := vs.items[idx].(WorkspaceItem)
			return a, a.doRequest("workspace.delete", map[string]any{"id": ws.ID}, func(_ json.RawMessage) {
				a.toast = fmt.Sprintf("Deleted workspace %q", ws.Name)
			})
		}
	}

	return a, nil
}

// --- Commands ---

func (a *App) connectCmd() tea.Cmd {
	return func() tea.Msg {
		if err := a.client.Connect(a.ctx); err != nil {
			return errMsg{err}
		}
		a.connected = true

		// Subscribe to events
		eventCh, err := a.client.Subscribe(a.ctx, []string{"tabs.*", "groups.*", "bookmarks.*"})
		if err == nil {
			a.eventCh = eventCh
		}

		return refreshMsg{}
	}
}

func (a *App) waitForEvent() tea.Cmd {
	if a.eventCh == nil {
		return nil
	}
	ch := a.eventCh
	return func() tea.Msg {
		evt, ok := <-ch
		if !ok {
			return nil
		}
		return eventMsg(evt)
	}
}

func (a *App) refreshCurrentView() tea.Cmd {
	if !a.connected {
		return nil
	}

	var action string
	switch a.view {
	case ViewTabs:
		action = "tabs.list"
	case ViewGroups:
		action = "groups.list"
	case ViewSessions:
		action = "sessions.list"
	case ViewCollections:
		action = "collections.list"
	case ViewTargets:
		action = "targets.list"
	case ViewBookmarks:
		action = "bookmarks.tree"
	case ViewWorkspaces:
		action = "workspace.list"
	case ViewSync:
		action = "sync.status"
	case ViewHistory:
		target := a.targetSelector()
		return func() tea.Msg {
			resp, err := a.client.Request(a.ctx, "history.search", map[string]string{"query": ""}, target)
			if err != nil {
				return errMsg{err}
			}
			return refreshMsg{payload: resp.Payload}
		}
	case ViewSearch:
		if a.searchActive && a.lastSearchQuery != "" {
			return a.performSearch(a.lastSearchQuery)
		}
		a.searchActive = false
		target := a.targetSelector()
		return func() tea.Msg {
			resp, err := a.client.Request(a.ctx, "search.saved.list", nil, target)
			if err != nil {
				return errMsg{err}
			}
			return refreshMsg{payload: resp.Payload}
		}
	case ViewDownloads:
		action = "downloads.list"
	default:
		return nil
	}

	target := a.targetSelector()
	return func() tea.Msg {
		resp, err := a.client.Request(a.ctx, action, nil, target)
		if err != nil {
			return errMsg{err}
		}
		return refreshMsg{payload: resp.Payload}
	}
}

func (a *App) applyRefresh(payload json.RawMessage) {
	if payload == nil {
		return
	}

	vs := a.currentView()

	switch a.view {
	case ViewTabs:
		tabs := parsePayload[TabItem](payload, "tabs")
		vs.items = make([]any, len(tabs))
		for i, t := range tabs {
			vs.items[i] = t
		}
		vs.itemCount = len(tabs)
	case ViewGroups:
		groups := parsePayload[GroupItem](payload, "groups")
		vs.items = make([]any, len(groups))
		for i, g := range groups {
			vs.items[i] = g
		}
		vs.itemCount = len(groups)
	case ViewSessions:
		sessions := parsePayload[SessionItem](payload, "sessions")
		vs.items = make([]any, len(sessions))
		for i, s := range sessions {
			vs.items[i] = s
		}
		vs.itemCount = len(sessions)
		if len(a.expandedSessions) > 0 {
			a.rebuildSessionItems()
		}
	case ViewCollections:
		collections := parsePayload[CollectionItem](payload, "collections")
		vs.items = make([]any, len(collections))
		for i, c := range collections {
			vs.items[i] = c
		}
		vs.itemCount = len(collections)
		if len(a.expandedCollections) > 0 {
			a.rebuildCollectionItems()
		}
	case ViewTargets:
		targets := parsePayload[TargetItem](payload, "targets")
		vs.items = make([]any, len(targets))
		for i, t := range targets {
			vs.items[i] = t
		}
		vs.itemCount = len(targets)
		// Auto-select default target
		for _, t := range targets {
			if t.IsDefault {
				a.selectedTarget = t.TargetID
			}
		}
	case ViewBookmarks:
		rawTree := parsePayload[BookmarkItem](payload, "tree")
		a.bookmarkTree = rawTree // save for re-flatten on fold/unfold
		flat := flattenBookmarkTreeWithCollapse(rawTree, 0, a.collapsedFolders)
		vs.items = make([]any, len(flat))
		for i, b := range flat {
			vs.items[i] = b
		}
		vs.itemCount = len(flat)
	case ViewWorkspaces:
		workspaces := parsePayload[WorkspaceItem](payload, "workspaces")
		vs.items = make([]any, len(workspaces))
		for i, w := range workspaces {
			vs.items[i] = w
		}
		vs.itemCount = len(workspaces)
	case ViewSync:
		// sync.status returns a single object, not an array
		var status SyncStatusItem
		if json.Unmarshal(payload, &status) == nil {
			vs.items = []any{status}
			vs.itemCount = 1
		}
	case ViewHistory:
		history := parsePayload[HistoryItem](payload, "history")
		vs.items = make([]any, len(history))
		for i, h := range history {
			vs.items[i] = h
		}
		vs.itemCount = len(history)
	case ViewSearch:
		if a.searchActive {
			results := parsePayload[SearchResultItem](payload, "results")
			vs.items = make([]any, len(results))
			for i, r := range results {
				vs.items[i] = r
			}
			vs.itemCount = len(results)
		} else {
			// Parse saved searches with nested query object
			var data struct {
				Searches []struct {
					ID    string `json:"id"`
					Name  string `json:"name"`
					Query struct {
						Query string `json:"query"`
					} `json:"query"`
					CreatedAt string `json:"createdAt"`
				} `json:"searches"`
			}
			json.Unmarshal(payload, &data)
			vs.items = make([]any, len(data.Searches))
			for i, s := range data.Searches {
				vs.items[i] = SavedSearchItem{
					ID:        s.ID,
					Name:      s.Name,
					QueryText: s.Query.Query,
					CreatedAt: s.CreatedAt,
				}
			}
			vs.itemCount = len(data.Searches)
		}
	case ViewDownloads:
		downloads := parsePayload[DownloadItem](payload, "downloads")
		vs.items = make([]any, len(downloads))
		for i, d := range downloads {
			vs.items[i] = d
		}
		vs.itemCount = len(downloads)
	}

	vs.clampCursor()
}

// autoSelectTargetAndRetry fetches the targets list, auto-selects the default
// (or first available) target, then refreshes the current view.
func (a *App) autoSelectTargetAndRetry() tea.Cmd {
	return func() tea.Msg {
		resp, err := a.client.Request(a.ctx, "targets.list", nil, nil)
		if err != nil {
			return errMsg{err}
		}
		targets := parsePayload[TargetItem](resp.Payload, "targets")
		for _, t := range targets {
			if t.IsDefault {
				a.selectedTarget = t.TargetID
			}
		}
		if a.selectedTarget == "" && len(targets) > 0 {
			a.selectedTarget = targets[0].TargetID
		}
		// Now retry the current view
		return targetRecoveredMsg{}
	}
}

func (a *App) targetSelector() *protocol.TargetSelector {
	if a.selectedTarget != "" {
		return &protocol.TargetSelector{TargetID: a.selectedTarget}
	}
	return nil
}

func (a *App) doRequest(action string, payload any, onSuccess func(json.RawMessage)) tea.Cmd {
	target := a.targetSelector()
	return func() tea.Msg {
		resp, err := a.client.Request(a.ctx, action, payload, target)
		if err != nil {
			return errMsg{err}
		}
		if onSuccess != nil {
			onSuccess(resp.Payload)
		}
		return refreshMsg{}
	}
}

func (a *App) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.MouseLeft:
		// Header is 2 lines, possible filter bar is 1 line
		headerLines := 2
		if a.mode == ModeFilter || a.mode == ModeCommand || a.mode == ModeNameInput {
			headerLines = 3
		}

		// Click on header (tab bar) — line 0
		if msg.Y == 0 {
			return a, a.handleTabBarClick(msg.X)
		}

		// Click on content area
		if msg.Y >= headerLines {
			vs := a.currentView()
			contentY := msg.Y - headerLines
			// Calculate scroll offset (same as renderContent)
			maxLines := a.contentHeight()
			start := 0
			if vs.cursor >= maxLines {
				start = vs.cursor - maxLines + 1
			}
			clickedItem := start + contentY
			if clickedItem >= 0 && clickedItem < vs.visibleCount() {
				if clickedItem == vs.cursor {
					// Double-click effect: activate on same item
					return a, a.handleEnter()
				}
				vs.cursor = clickedItem
			}
			return a, nil
		}

	case tea.MouseWheelUp:
		a.moveCursor(-3)
		return a, nil

	case tea.MouseWheelDown:
		a.moveCursor(3)
		return a, nil
	}
	return a, nil
}

func (a *App) handleTabBarClick(x int) tea.Cmd {
	// Tab bar layout: "CTM  Targets  Tabs  Groups  Sessions  Collections  Bookmarks  Workspaces  Sync"
	// Map click X position to view type by measuring cumulative widths
	tabs := []struct {
		name string
		view ViewType
	}{
		{"CTM", ViewTargets}, // clicking CTM goes to Targets
		{"Targets", ViewTargets},
		{"Tabs", ViewTabs},
		{"Groups", ViewGroups},
		{"Sessions", ViewSessions},
		{"Collections", ViewCollections},
		{"Bookmarks", ViewBookmarks},
		{"Workspaces", ViewWorkspaces},
		{"Sync", ViewSync},
		{"History", ViewHistory},
		{"Search", ViewSearch},
		{"Downloads", ViewDownloads},
	}

	pos := 0
	for _, t := range tabs {
		end := pos + len(t.name) + 2 // +2 for padding
		if x >= pos && x < end {
			a.view = t.view
			return a.refreshCurrentView()
		}
		pos = end
	}
	return nil
}

func (a *App) handleEnter() tea.Cmd {
	vs := a.currentView()
	if vs.visibleCount() == 0 {
		return nil
	}
	idx := vs.realIndex(vs.cursor)

	switch a.view {
	case ViewTabs:
		if idx < len(vs.items) {
			tab := vs.items[idx].(TabItem)
			return a.doRequest("tabs.activate", map[string]any{"tabId": tab.ID, "focus": true}, nil)
		}
	case ViewGroups:
		if idx < len(vs.items) {
			g := vs.items[idx].(GroupItem)
			// Toggle collapsed state
			return a.doRequest("groups.update", map[string]any{
				"groupId":   g.ID,
				"collapsed": !g.Collapsed,
			}, func(_ json.RawMessage) {
				state := "expanded"
				if !g.Collapsed {
					state = "collapsed"
				}
				a.toast = fmt.Sprintf("Group %q %s", g.Title, state)
			})
		}
	case ViewTargets:
		if idx < len(vs.items) {
			t := vs.items[idx].(TargetItem)
			a.selectedTarget = t.TargetID
			a.view = ViewTabs
			return a.refreshCurrentView()
		}
	case ViewBookmarks:
		if idx < len(vs.items) {
			bm := vs.items[idx].(BookmarkItem)
			if bm.IsFolder {
				// Toggle fold/unfold
				wasCollapsed := a.collapsedFolders[bm.ID]
				if wasCollapsed {
					delete(a.collapsedFolders, bm.ID)
				} else {
					a.collapsedFolders[bm.ID] = true
				}
				a.reflattenBookmarks()
				a.toast = fmt.Sprintf("folder %q: %v→%v (children=%d, total=%d)",
					bm.Title, wasCollapsed, a.collapsedFolders[bm.ID], len(bm.Children), len(a.views[ViewBookmarks].items))
				return tea.Tick(5*time.Second, func(time.Time) tea.Msg { return toastClearMsg{} })
			}
			// For bookmark links, open URL via tabs.open
			if bm.URL != "" {
				return a.doRequest("tabs.open", map[string]any{"url": bm.URL, "active": true, "focus": true}, nil)
			}
		}
	case ViewHistory:
		if idx < len(vs.items) {
			h := vs.items[idx].(HistoryItem)
			if h.URL != "" {
				return a.doRequest("tabs.open", map[string]any{"url": h.URL, "active": true, "focus": true}, nil)
			}
		}
	case ViewSearch:
		if idx < len(vs.items) {
			switch item := vs.items[idx].(type) {
			case SavedSearchItem:
				// Execute saved search
				return a.performSearch(item.QueryText)
			case SearchResultItem:
				// Open result: if URL exists, open it; otherwise navigate to view
				if item.URL != "" {
					return a.doRequest("tabs.open", map[string]any{"url": item.URL, "active": true, "focus": true}, nil)
				}
				// Navigate to the appropriate view for non-URL results
				switch item.Kind {
				case "session":
					a.view = ViewSessions
				case "collection":
					a.view = ViewCollections
				case "workspace":
					a.view = ViewWorkspaces
				default:
					a.toast = fmt.Sprintf("%s: %s", item.Kind, item.Title)
					return a.showToast(a.toast)
				}
				return a.refreshCurrentView()
			}
		}
	case ViewWorkspaces:
		if idx < len(vs.items) {
			ws := vs.items[idx].(WorkspaceItem)
			wsID, wsName := ws.ID, ws.Name
			return a.doRequest("workspace.get", map[string]any{"id": wsID}, func(payload json.RawMessage) {
				var detail struct {
					Workspace struct {
						Sessions    []string `json:"sessions"`
						Collections []string `json:"collections"`
						Description string   `json:"description"`
						Status      string   `json:"status"`
					} `json:"workspace"`
				}
				json.Unmarshal(payload, &detail)
				w := detail.Workspace
				a.toast = fmt.Sprintf("%s: %d sessions, %d collections, status=%s",
					wsName, len(w.Sessions), len(w.Collections), w.Status)
			})
		}
	case ViewSessions:
		if idx < len(vs.items) {
			switch item := vs.items[idx].(type) {
			case SessionItem:
				if _, ok := a.expandedSessions[item.Name]; ok {
					delete(a.expandedSessions, item.Name)
					a.rebuildSessionItems()
					return nil
				}
				return a.doRequest("sessions.get", map[string]string{"name": item.Name}, func(payload json.RawMessage) {
					tabs := a.parseSessionTabs(payload, item.Name)
					a.expandedSessions[item.Name] = tabs
					a.rebuildSessionItems()
				})
			case NestedTabItem:
				return a.openSingleTab(item)
			}
		}
	case ViewCollections:
		if idx < len(vs.items) {
			switch item := vs.items[idx].(type) {
			case CollectionItem:
				if _, ok := a.expandedCollections[item.Name]; ok {
					delete(a.expandedCollections, item.Name)
					a.rebuildCollectionItems()
					return nil
				}
				return a.doRequest("collections.get", map[string]string{"name": item.Name}, func(payload json.RawMessage) {
					tabs := a.parseCollectionTabs(payload, item.Name)
					a.expandedCollections[item.Name] = tabs
					a.rebuildCollectionItems()
				})
			case NestedTabItem:
				return a.openSingleTab(item)
			}
		}
	}
	return nil
}

func (a *App) closeTabs() tea.Cmd {
	vs := a.views[ViewTabs]
	var cmds []tea.Cmd

	if len(vs.selected) > 0 {
		for idx := range vs.selected {
			if idx < len(vs.items) {
				tab := vs.items[idx].(TabItem)
				cmds = append(cmds, a.doRequest("tabs.close", map[string]any{"tabId": tab.ID}, nil))
			}
		}
		vs.selected = make(map[int]bool)
	} else if vs.cursor < vs.visibleCount() {
		idx := vs.realIndex(vs.cursor)
		tab := vs.items[idx].(TabItem)
		cmds = append(cmds, a.doRequest("tabs.close", map[string]any{"tabId": tab.ID}, nil))
	}

	if len(cmds) > 0 {
		return tea.Batch(cmds...)
	}
	return nil
}

func (a *App) toggleTabMute() tea.Cmd {
	vs := a.views[ViewTabs]
	if vs.cursor >= vs.visibleCount() {
		return nil
	}
	idx := vs.realIndex(vs.cursor)
	if idx >= len(vs.items) {
		return nil
	}
	tab := vs.items[idx].(TabItem)
	return a.doRequest("tabs.mute", map[string]any{"tabId": tab.ID}, func(_ json.RawMessage) {
		a.toast = fmt.Sprintf("Toggled mute on %q", tab.Title)
	})
}

func (a *App) toggleTabPin() tea.Cmd {
	vs := a.views[ViewTabs]
	if vs.cursor >= vs.visibleCount() {
		return nil
	}
	idx := vs.realIndex(vs.cursor)
	if idx >= len(vs.items) {
		return nil
	}
	tab := vs.items[idx].(TabItem)
	return a.doRequest("tabs.pin", map[string]any{"tabId": tab.ID}, func(_ json.RawMessage) {
		a.toast = fmt.Sprintf("Toggled pin on %q", tab.Title)
	})
}

func (a *App) currentTab() (TabItem, bool) {
	vs := a.views[ViewTabs]
	if vs.cursor >= vs.visibleCount() {
		return TabItem{}, false
	}
	idx := vs.realIndex(vs.cursor)
	if idx >= len(vs.items) {
		return TabItem{}, false
	}
	return vs.items[idx].(TabItem), true
}

func (a *App) fetchTabPreview() tea.Cmd {
	tab, ok := a.currentTab()
	if !ok || tab.URL == "" {
		return nil
	}

	target := a.targetSelector()

	if a.previewMode == 1 {
		// Text preview: inject content script to extract page text
		tabID := tab.ID
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			resp, err := a.client.Request(ctx, "tabs.getText", map[string]any{"tabId": tabID}, target)
			if err != nil {
				return previewTextMsg{tabID: tabID, text: "(failed to get text: " + err.Error() + ")"}
			}
			var result struct {
				Text string `json:"text"`
			}
			json.Unmarshal(resp.Payload, &result)
			text := result.Text
			if text == "" {
				text = "(no text content)"
			}
			return previewTextMsg{tabID: tabID, text: text}
		}
	}

	if a.previewMode == 2 {
		// Screenshot preview: capture visible tab area
		tabID := tab.ID
		windowID := tab.WindowID
		if _, ok := a.previewImage[tabID]; ok {
			return nil // already cached
		}
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			resp, err := a.client.Request(ctx, "tabs.capture", map[string]any{
				"tabId": tabID, "windowId": windowID, "format": "png",
			}, target)
			if err != nil {
				return previewImageMsg{tabID: tabID, dataURL: "(capture failed: " + err.Error() + ")"}
			}
			var result struct {
				DataURL string `json:"dataUrl"`
			}
			json.Unmarshal(resp.Payload, &result)
			if result.DataURL == "" {
				return previewImageMsg{tabID: tabID, dataURL: "(empty capture)"}
			}
			// Save to temp file for Kitty graphics protocol
			idx := strings.Index(result.DataURL, ",")
			if idx < 0 {
				return previewImageMsg{tabID: tabID, dataURL: "(invalid dataUrl)"}
			}
			imgBytes, err := base64.StdEncoding.DecodeString(result.DataURL[idx+1:])
			if err != nil {
				return previewImageMsg{tabID: tabID, dataURL: "(decode error)"}
			}
			tmpFile := fmt.Sprintf("/tmp/ctm-screenshot-%d.png", tabID)
			os.WriteFile(tmpFile, imgBytes, 0644)
			return previewImageMsg{tabID: tabID, dataURL: tmpFile}
		}
	}

	return nil
}

func (a *App) openW3M() tea.Cmd {
	tab, ok := a.currentTab()
	if !ok || tab.URL == "" {
		return nil
	}

	// Check for w3m, fallback to lynx
	bin := "w3m"
	if _, err := exec.LookPath("w3m"); err != nil {
		if _, err2 := exec.LookPath("lynx"); err2 == nil {
			bin = "lynx"
		} else {
			a.errorMsg = "w3m or lynx not found. Install with: brew install w3m"
			return nil
		}
	}

	c := exec.Command(bin, tab.URL)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{fmt.Errorf("w3m: %w", err)}
		}
		return refreshMsg{}
	})
}

// captureAndOpenExternal captures a screenshot and opens it externally.
// Used by P key for full-screen viewing outside the TUI.
func (a *App) captureAndOpenExternal() tea.Cmd {
	tab, ok := a.currentTab()
	if !ok {
		return nil
	}
	// If already cached from preview, just open the file
	if imgPath, ok := a.previewImage[tab.ID]; ok && !strings.HasPrefix(imgPath, "(") {
		exec.Command("open", imgPath).Start()
		return func() tea.Msg {
			return toastMsg(fmt.Sprintf("Opened: %s", imgPath))
		}
	}
	target := a.targetSelector()
	tabID := tab.ID
	windowID := tab.WindowID
	a.toast = "Capturing screenshot..."
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		resp, err := a.client.Request(ctx, "tabs.capture", map[string]any{
			"tabId": tabID, "windowId": windowID, "format": "png",
		}, target)
		if err != nil {
			return errMsg{fmt.Errorf("capture: %w", err)}
		}
		var result struct {
			DataURL string `json:"dataUrl"`
		}
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			return errMsg{fmt.Errorf("capture parse: %w", err)}
		}
		if result.DataURL == "" {
			return errMsg{fmt.Errorf("capture: empty response")}
		}
		idx := strings.Index(result.DataURL, ",")
		if idx < 0 {
			return errMsg{fmt.Errorf("capture: invalid dataUrl")}
		}
		imgBytes, err := base64.StdEncoding.DecodeString(result.DataURL[idx+1:])
		if err != nil {
			return errMsg{fmt.Errorf("capture decode: %w", err)}
		}
		tmpFile := fmt.Sprintf("/tmp/ctm-screenshot-%d.png", tabID)
		if err := os.WriteFile(tmpFile, imgBytes, 0644); err != nil {
			return errMsg{fmt.Errorf("capture write: %w", err)}
		}
		exec.Command("open", tmpFile).Start()
		return toastMsg(fmt.Sprintf("Screenshot saved: %s", tmpFile))
	}
}

// parseSessionTabs extracts tabs from a sessions.get response.
func (a *App) parseSessionTabs(payload json.RawMessage, sessionName string) []NestedTabItem {
	var data struct {
		Session struct {
			Windows []struct {
				Tabs []struct {
					URL    string `json:"url"`
					Title  string `json:"title"`
					Pinned bool   `json:"pinned"`
				} `json:"tabs"`
			} `json:"windows"`
		} `json:"session"`
	}
	json.Unmarshal(payload, &data)
	var tabs []NestedTabItem
	for _, w := range data.Session.Windows {
		for _, t := range w.Tabs {
			tabs = append(tabs, NestedTabItem{URL: t.URL, Title: t.Title, Pinned: t.Pinned, ParentName: sessionName})
		}
	}
	return tabs
}

// parseCollectionTabs extracts tabs from a collections.get response.
func (a *App) parseCollectionTabs(payload json.RawMessage, collName string) []NestedTabItem {
	var data struct {
		Collection struct {
			Items []struct {
				URL   string `json:"url"`
				Title string `json:"title"`
			} `json:"items"`
		} `json:"collection"`
	}
	json.Unmarshal(payload, &data)
	var tabs []NestedTabItem
	for _, item := range data.Collection.Items {
		tabs = append(tabs, NestedTabItem{URL: item.URL, Title: item.Title, ParentName: collName})
	}
	return tabs
}

// rebuildSessionItems rebuilds the sessions view items list with expanded tabs interleaved.
func (a *App) rebuildSessionItems() {
	vs := a.views[ViewSessions]
	var newItems []any
	for _, item := range vs.items {
		if s, ok := item.(SessionItem); ok {
			newItems = append(newItems, s)
			if tabs, expanded := a.expandedSessions[s.Name]; expanded {
				for _, t := range tabs {
					newItems = append(newItems, t)
				}
			}
		}
		// Skip existing NestedTabItem (will be re-inserted from expandedSessions)
	}
	vs.items = newItems
	vs.itemCount = len(newItems)
	vs.clampCursor()
}

// rebuildCollectionItems rebuilds the collections view items list with expanded tabs interleaved.
func (a *App) rebuildCollectionItems() {
	vs := a.views[ViewCollections]
	var newItems []any
	for _, item := range vs.items {
		if c, ok := item.(CollectionItem); ok {
			newItems = append(newItems, c)
			if tabs, expanded := a.expandedCollections[c.Name]; expanded {
				for _, t := range tabs {
					newItems = append(newItems, t)
				}
			}
		}
	}
	vs.items = newItems
	vs.itemCount = len(newItems)
	vs.clampCursor()
}

// openSingleTab opens a single tab URL in the browser and focuses the window.
func (a *App) openSingleTab(tab NestedTabItem) tea.Cmd {
	return a.doRequest("tabs.open", map[string]any{"url": tab.URL, "focus": true}, func(_ json.RawMessage) {
		a.toast = fmt.Sprintf("Opened: %s", tab.Title)
	})
}

func (a *App) handleRestore() tea.Cmd {
	vs := a.currentView()
	idx := vs.realIndex(vs.cursor)

	// If cursor is on a NestedTabItem, open just that tab
	if idx < len(vs.items) {
		if tab, ok := vs.items[idx].(NestedTabItem); ok {
			return a.openSingleTab(tab)
		}
	}

	name := a.currentItemName()
	if name == "" {
		return nil
	}

	switch a.view {
	case ViewSessions:
		return a.doRequest("sessions.restore", map[string]string{"name": name}, func(_ json.RawMessage) {
			a.toast = fmt.Sprintf("Restored session %q", name)
		})
	case ViewCollections:
		return a.doRequest("collections.restore", map[string]string{"name": name}, func(_ json.RawMessage) {
			a.toast = fmt.Sprintf("Restored collection %q", name)
		})
	}
	return nil
}

func (a *App) saveSession(name string) tea.Cmd {
	return a.doRequest("sessions.save", map[string]string{"name": name}, func(_ json.RawMessage) {
		a.toast = fmt.Sprintf("Saved session %q", name)
	})
}

func (a *App) createCollection(name string) tea.Cmd {
	return a.doRequest("collections.create", map[string]string{"name": name}, func(_ json.RawMessage) {
		a.toast = fmt.Sprintf("Created collection %q", name)
	})
}

func (a *App) switchWorkspace() tea.Cmd {
	vs := a.views[ViewWorkspaces]
	if vs.visibleCount() == 0 {
		return nil
	}
	idx := vs.realIndex(vs.cursor)
	if idx >= len(vs.items) {
		return nil
	}
	ws := vs.items[idx].(WorkspaceItem)
	return a.doRequest("workspace.switch", map[string]any{"id": ws.ID}, func(payload json.RawMessage) {
		var result struct {
			TabsClosed int `json:"tabsClosed"`
			TabsOpened int `json:"tabsOpened"`
		}
		json.Unmarshal(payload, &result)
		a.toast = fmt.Sprintf("Switched to %q (closed %d, opened %d)", ws.Name, result.TabsClosed, result.TabsOpened)
	})
}

func (a *App) performSearch(query string) tea.Cmd {
	a.searchActive = true
	a.lastSearchQuery = query
	target := a.targetSelector()
	return func() tea.Msg {
		resp, err := a.client.Request(a.ctx, "search.query", map[string]any{
			"query": query,
			"limit": 50,
		}, target)
		if err != nil {
			return errMsg{err}
		}
		return refreshMsg{payload: resp.Payload}
	}
}

func (a *App) cancelDownload() tea.Cmd {
	vs := a.views[ViewDownloads]
	if vs.cursor >= vs.visibleCount() {
		return nil
	}
	idx := vs.realIndex(vs.cursor)
	if idx >= len(vs.items) {
		return nil
	}
	dl := vs.items[idx].(DownloadItem)
	return a.doRequest("downloads.cancel", map[string]any{"id": dl.ID}, func(_ json.RawMessage) {
		a.toast = fmt.Sprintf("Cancelled download: %s", truncate(dl.Filename, 30))
	})
}

func (a *App) setDefaultTarget() tea.Cmd {
	vs := a.views[ViewTargets]
	if vs.cursor >= vs.visibleCount() {
		return nil
	}
	idx := vs.realIndex(vs.cursor)
	if idx >= len(vs.items) {
		return nil
	}
	t := vs.items[idx].(TargetItem)
	return a.doRequest("targets.default", map[string]string{"targetId": t.TargetID}, func(_ json.RawMessage) {
		a.toast = fmt.Sprintf("Default target: %s", t.TargetID)
	})
}

func (a *App) showToast(msg string) tea.Cmd {
	a.toast = msg
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return toastClearMsg{} })
}

func (a *App) executeCommand(cmd string) tea.Cmd {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil
	}

	switch parts[0] {
	case "q", "quit":
		a.cancel()
		return tea.Quit
	case "help":
		a.mode = ModeHelp
		return nil
	case "target":
		a.view = ViewTargets
		return a.refreshCurrentView()
	case "save":
		if len(parts) > 1 {
			return a.saveSession(parts[1])
		}
	case "restore":
		if len(parts) > 1 {
			return a.doRequest("sessions.restore", map[string]string{"name": parts[1]}, func(_ json.RawMessage) {
				a.toast = fmt.Sprintf("Restored %q", parts[1])
			})
		}
	}
	return nil
}

// --- Navigation helpers ---

func (a *App) currentView() *ViewState {
	return a.views[a.view]
}

func (a *App) moveCursor(delta int) {
	vs := a.currentView()
	oldCursor := vs.cursor
	vs.cursor += delta
	vs.clampCursor()
	// Track cursor change for preview refresh
	if a.view == ViewTabs && vs.cursor != oldCursor && a.previewMode > 0 {
		a.pendingPreviewFetch = true
	}
}

func (a *App) toggleSelect() {
	vs := a.currentView()
	idx := vs.realIndex(vs.cursor)
	if vs.selected[idx] {
		delete(vs.selected, idx)
	} else {
		vs.selected[idx] = true
	}
	a.moveCursor(1)
}

func (a *App) selectAll() {
	vs := a.currentView()
	for i := 0; i < vs.itemCount; i++ {
		vs.selected[i] = true
	}
}

func (a *App) clearSelection() {
	vs := a.currentView()
	vs.selected = make(map[int]bool)
}

func (a *App) nextView(delta int) {
	views := []ViewType{ViewTargets, ViewTabs, ViewGroups, ViewSessions, ViewCollections, ViewBookmarks, ViewWorkspaces, ViewSync, ViewHistory, ViewSearch, ViewDownloads}
	for i, v := range views {
		if v == a.view {
			next := (i + delta + len(views)) % len(views)
			a.view = views[next]
			return
		}
	}
}

func (a *App) contentHeight() int {
	h := a.height - 3
	if a.mode == ModeFilter || a.mode == ModeCommand || a.mode == ModeNameInput {
		h--
	}
	if h < 1 {
		h = 1
	}
	return h
}

func (a *App) currentItemName() string {
	vs := a.currentView()
	if vs.visibleCount() == 0 {
		return ""
	}
	idx := vs.realIndex(vs.cursor)
	if idx >= len(vs.items) {
		return ""
	}

	switch v := vs.items[idx].(type) {
	case SessionItem:
		return v.Name
	case CollectionItem:
		return v.Name
	case WorkspaceItem:
		return v.Name
	case NestedTabItem:
		return "" // nested tabs cannot be deleted directly
	}
	return ""
}

func (a *App) applyFilter() {
	vs := a.currentView()
	if a.filterText == "" {
		vs.filtered = nil
		return
	}

	query := strings.ToLower(a.filterText)
	vs.filtered = []int{}
	for i, item := range vs.items {
		if matchesFilter(item, query) {
			vs.filtered = append(vs.filtered, i)
		}
	}
	vs.clampCursor()
}

func (a *App) clearFilter() {
	vs := a.currentView()
	vs.filtered = nil
	vs.clampCursor()
}

func matchesFilter(item any, query string) bool {
	switch v := item.(type) {
	case TabItem:
		return strings.Contains(strings.ToLower(v.Title), query) ||
			strings.Contains(strings.ToLower(v.URL), query)
	case GroupItem:
		return strings.Contains(strings.ToLower(v.Title), query)
	case SessionItem:
		return strings.Contains(strings.ToLower(v.Name), query)
	case CollectionItem:
		return strings.Contains(strings.ToLower(v.Name), query)
	case NestedTabItem:
		return strings.Contains(strings.ToLower(v.Title), query) ||
			strings.Contains(strings.ToLower(v.URL), query)
	case TargetItem:
		return strings.Contains(strings.ToLower(v.Label), query) ||
			strings.Contains(strings.ToLower(v.TargetID), query)
	case BookmarkItem:
		return strings.Contains(strings.ToLower(v.Title), query) ||
			strings.Contains(strings.ToLower(v.URL), query)
	case WorkspaceItem:
		return strings.Contains(strings.ToLower(v.Name), query)
	case SyncStatusItem:
		return strings.Contains(strings.ToLower(v.SyncDir), query)
	case HistoryItem:
		return strings.Contains(strings.ToLower(v.Title), query) ||
			strings.Contains(strings.ToLower(v.URL), query)
	case SearchResultItem:
		return strings.Contains(strings.ToLower(v.Title), query) ||
			strings.Contains(strings.ToLower(v.URL), query) ||
			strings.Contains(strings.ToLower(v.Kind), query)
	case SavedSearchItem:
		return strings.Contains(strings.ToLower(v.Name), query) ||
			strings.Contains(strings.ToLower(v.QueryText), query)
	case DownloadItem:
		return strings.Contains(strings.ToLower(v.Filename), query) ||
			strings.Contains(strings.ToLower(v.URL), query)
	}
	return false
}

// --- Rendering ---

func (a *App) renderHeader() string {
	views := []ViewType{ViewTargets, ViewTabs, ViewGroups, ViewSessions, ViewCollections, ViewBookmarks, ViewWorkspaces, ViewSync, ViewHistory, ViewSearch, ViewDownloads}
	var tabs []string
	for _, v := range views {
		label := v.String()
		if v == a.view {
			tabs = append(tabs, styleActiveTab.Render(label))
		} else {
			tabs = append(tabs, styleInactiveTab.Render(label))
		}
	}

	line1 := styleHeader.Render("CTM") + "  " + strings.Join(tabs, "")

	// Connection status — right-aligned on the same line
	var connStatus string
	if !a.connected {
		connStatus = styleDanger.Render("disconnected")
	} else if a.selectedTarget != "" {
		connStatus = styleAccent.Render("target: " + a.selectedTarget)
	} else {
		connStatus = styleAccent.Render("connected")
	}
	gap := a.width - lipgloss.Width(line1) - lipgloss.Width(connStatus)
	if gap < 2 {
		gap = 2
	}
	line1 = line1 + strings.Repeat(" ", gap) + connStatus

	// Separator line
	line2 := styleDim.Render(strings.Repeat("─", a.width))

	return line1 + "\n" + line2
}

func (a *App) renderContent(maxLines int) string {
	// Split view for Tabs, Sessions, Collections
	if a.view == ViewTabs || a.view == ViewSessions || a.view == ViewCollections {
		return a.renderSplitContent(maxLines)
	}

	vs := a.currentView()
	if vs.visibleCount() == 0 {
		return styleDim.Render("  (empty)")
	}

	var lines []string
	start := 0
	if vs.cursor >= maxLines {
		start = vs.cursor - maxLines + 1
	}

	for i := start; i < vs.visibleCount() && len(lines) < maxLines; i++ {
		idx := vs.realIndex(i)
		if idx >= len(vs.items) {
			continue
		}

		isCursor := i == vs.cursor
		isSelected := vs.selected[idx]

		prefix := "  "
		if isCursor {
			prefix = styleCursor.Render("> ")
		}
		if isSelected {
			prefix = styleSelected.Render("* ")
		}

		line := a.renderItem(vs.items[idx])
		if isCursor {
			line = styleRowActive.Render(line)
		}
		lines = append(lines, prefix+line)
	}

	return strings.Join(lines, "\n")
}

// renderSplitContent renders a two-panel layout for Sessions/Collections.
// Left panel (~1/3): item list. Right panel (~2/3): preview of selected item.
func (a *App) renderSplitContent(maxLines int) string {
	vs := a.currentView()
	if vs.visibleCount() == 0 {
		return styleDim.Render("  (empty)")
	}

	// Panel widths: left ~1/3, separator 3 chars (" │ "), right ~2/3
	leftW := a.width / 3
	if leftW < 20 {
		leftW = 20
	}
	separatorW := 3
	rightW := a.width - leftW - separatorW
	if rightW < 20 {
		rightW = 20
	}

	leftLines := a.renderListPanel(vs, leftW, maxLines)
	rightLines := a.renderPreviewPanel(vs, rightW, maxLines)

	separator := styleDim.Render(" \u2502 ") // " │ "

	var result []string
	for i := 0; i < maxLines; i++ {
		left := ""
		if i < len(leftLines) {
			left = leftLines[i]
		}
		right := ""
		if i < len(rightLines) {
			right = rightLines[i]
		}
		left = rwPadRight(left, leftW)
		result = append(result, left+separator+right)
	}
	return strings.Join(result, "\n")
}

// renderListPanel renders the left panel item list for split view.
func (a *App) renderListPanel(vs *ViewState, panelW, maxLines int) []string {
	var lines []string
	start := 0
	if vs.cursor >= maxLines {
		start = vs.cursor - maxLines + 1
	}

	// Content width = panelW minus 2 chars for prefix ("  " or "> " or "* ")
	contentW := panelW - 2
	if contentW < 10 {
		contentW = 10
	}

	for i := start; i < vs.visibleCount() && len(lines) < maxLines; i++ {
		idx := vs.realIndex(i)
		if idx >= len(vs.items) {
			continue
		}

		isCursor := i == vs.cursor
		isSelected := vs.selected[idx]

		prefix := "  "
		if isCursor {
			prefix = styleCursor.Render("> ")
		}
		if isSelected {
			prefix = styleSelected.Render("* ")
		}

		line := a.renderListItem(vs.items[idx], contentW)
		if isCursor {
			line = styleRowActive.Render(line)
		}
		lines = append(lines, prefix+line)
	}
	return lines
}

// renderListItem renders a compact item for the left panel of split view.
func (a *App) renderListItem(item any, maxW int) string {
	switch v := item.(type) {
	case TabItem:
		// [icon] title
		var icon string
		if v.Pinned {
			icon = lipgloss.NewStyle().Foreground(colorOrange).Render("\u2295") + " "
		} else if v.Active {
			icon = styleAccent.Render("\u25cf") + " "
		} else {
			icon = styleDim.Render("\u25cb") + " "
		}
		titleW := maxW - 3 // icon takes ~3 chars
		if titleW < 5 {
			titleW = 5
		}
		title := rwTruncate(v.Title, titleW, "...")
		var flags string
		if v.Muted {
			flags = styleFlags.Render(" \U0001f507")
		}
		return icon + styleTitle.Render(title) + flags

	case SessionItem:
		// "session_name  (24 tabs)"
		countStr := fmt.Sprintf("(%d tabs)", v.TabCount)
		nameW := maxW - runewidth.StringWidth(countStr) - 2 // 2 for gap
		if nameW < 5 {
			nameW = 5
		}
		name := rwTruncate(v.Name, nameW, "...")
		pad := nameW - runewidth.StringWidth(name)
		if pad < 0 {
			pad = 0
		}
		return stylePurple.Render(name) + strings.Repeat(" ", pad) + "  " + styleDim.Render(countStr)

	case CollectionItem:
		// "collection_name  (5 items)"
		countStr := fmt.Sprintf("(%d items)", v.ItemCount)
		nameW := maxW - runewidth.StringWidth(countStr) - 2
		if nameW < 5 {
			nameW = 5
		}
		name := rwTruncate(v.Name, nameW, "...")
		pad := nameW - runewidth.StringWidth(name)
		if pad < 0 {
			pad = 0
		}
		return stylePurple.Render(name) + strings.Repeat(" ", pad) + "  " + styleDim.Render(countStr)

	case NestedTabItem:
		indent := "  "
		bullet := styleDim.Render("\u2022 ")
		domainW := maxW * 25 / 100
		if domainW > 20 {
			domainW = 20
		}
		titleW := maxW - 6 - domainW - 2 // -6 for indent+bullet, -2 for gap
		if titleW < 5 {
			titleW = 5
		}
		title := rwTruncate(v.Title, titleW, "...")
		domain := extractDomain(v.URL)
		domain = rwTruncate(domain, domainW, "...")
		titlePad := titleW - runewidth.StringWidth(title)
		if titlePad < 0 {
			titlePad = 0
		}
		return indent + bullet + styleTitle.Render(title) + strings.Repeat(" ", titlePad) + "  " + styleURLDomain.Render(domain)
	}
	return fmt.Sprintf("%v", item)
}

// renderPreviewPanel renders the right panel preview for the currently selected item.
func (a *App) renderPreviewPanel(vs *ViewState, panelW, maxLines int) []string {
	if vs.visibleCount() == 0 {
		return nil
	}

	idx := vs.realIndex(vs.cursor)
	if idx >= len(vs.items) {
		return nil
	}

	var lines []string
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)

	switch v := vs.items[idx].(type) {
	case TabItem:
		// Title bar (always shown)
		title := rwTruncate(v.Title, panelW, "...")
		lines = append(lines, headerStyle.Render(title))
		lines = append(lines, styleDim.Render(strings.Repeat("\u2500", min(runewidth.StringWidth(v.Title), panelW))))

		switch a.previewMode {
		case 1: // Text preview
			modes := styleDim.Render("[v:info] ") + styleAccent.Render("[text]") + styleDim.Render("  [s:screenshot]")
			lines = append(lines, modes)
			lines = append(lines, "")
			if text, ok := a.previewText[v.ID]; ok {
				for _, line := range strings.Split(text, "\n") {
					if len(lines) >= maxLines-1 {
						break
					}
					lines = append(lines, rwTruncate(line, panelW, ""))
				}
			} else {
				lines = append(lines, styleWarnText(fmt.Sprintf("Loading text... (tabId=%d, target=%s)", v.ID, a.selectedTarget)))
			}

		case 2: // Screenshot preview
			modes := styleDim.Render("[v:info]  [v:text]  ") + styleAccent.Render("[screenshot]")
			lines = append(lines, modes)
			lines = append(lines, "")
			if imgPath, ok := a.previewImage[v.ID]; ok {
				if strings.HasPrefix(imgPath, "(") {
					// Error message
					lines = append(lines, styleWarnText(imgPath))
				} else {
					// Kitty graphics protocol: display image from file
					// \033_Gf=100,a=T,t=f,c=COLS,r=ROWS,q=2;\033\\
					cols := panelW - 2
					rows := maxLines - 5
					if rows < 4 {
						rows = 4
					}
					pathB64 := base64.StdEncoding.EncodeToString([]byte(imgPath))
					kittySeq := fmt.Sprintf("\033_Gf=100,a=T,t=f,c=%d,r=%d,q=2;%s\033\\", cols, rows, pathB64)
					lines = append(lines, kittySeq)
					// Pad remaining lines so image has space
					for i := 0; i < rows && len(lines) < maxLines-1; i++ {
						lines = append(lines, "")
					}
				}
			} else {
				lines = append(lines, styleWarnText("Capturing screenshot..."))
			}

		default: // Info mode (0)
			modes := styleAccent.Render("[info]") + styleDim.Render(" [v:text]  [s:screenshot]")
			lines = append(lines, modes)

			// URL
			lines = append(lines, "")
			urlStr := rwTruncate(v.URL, panelW, "...")
			lines = append(lines, styleURLDomain.Render(urlStr))

			// Properties
			lines = append(lines, "")
			if v.Active {
				lines = append(lines, styleTitle.Render("Status:   ")+styleAccent.Render("active"))
			}
			if v.Pinned {
				lines = append(lines, styleTitle.Render("Pinned:   ")+styleFlags.Render("yes"))
			}
			if v.Muted {
				lines = append(lines, styleTitle.Render("Muted:    ")+styleFlags.Render("yes"))
			}
			if v.GroupID >= 0 {
				lines = append(lines, styleTitle.Render("Group:    ")+styleInfo.Render(fmt.Sprintf("%d", v.GroupID)))
			}
			domain := extractDomain(v.URL)
			if domain != "" {
				lines = append(lines, styleTitle.Render("Domain:   ")+styleURLDomain.Render(domain))
			}

			// Key hints
			lines = append(lines, "")
			lines = append(lines, styleDim.Render("Enter:activate  x:close  P:w3m"))
			lines = append(lines, styleDim.Render("m:mute  p:pin  y·:copy  v:preview"))
		}

	case SessionItem:
		// Title
		title := rwTruncate(v.Name, panelW, "...")
		lines = append(lines, headerStyle.Render(title))
		lines = append(lines, styleDim.Render(strings.Repeat("\u2500", min(runewidth.StringWidth(v.Name), panelW)))) // ─

		// Stats
		lines = append(lines, "")
		lines = append(lines, styleTitle.Render("Tabs:     ")+styleInfo.Render(fmt.Sprintf("%d", v.TabCount)))
		lines = append(lines, styleTitle.Render("Windows:  ")+styleInfo.Render(fmt.Sprintf("%d", v.WindowCount)))
		lines = append(lines, styleTitle.Render("Groups:   ")+styleInfo.Render(fmt.Sprintf("%d", v.GroupCount)))

		// Timestamps
		lines = append(lines, "")
		if v.CreatedAt != "" {
			ts := v.CreatedAt
			if len(ts) > 19 {
				ts = ts[:19]
			}
			lines = append(lines, styleTitle.Render("Created:  ")+styleDim.Render(ts))
		}

		// Source target
		if v.SourceTarget != "" {
			src := rwTruncate(v.SourceTarget, panelW-10, "...")
			lines = append(lines, styleTitle.Render("Source:   ")+styleDim.Render(src))
		}

		// Key hints
		lines = append(lines, "")
		lines = append(lines, styleDim.Render("o:restore  n:save  D-D:delete"))

	case CollectionItem:
		// Title
		title := rwTruncate(v.Name, panelW, "...")
		lines = append(lines, headerStyle.Render(title))
		lines = append(lines, styleDim.Render(strings.Repeat("\u2500", min(runewidth.StringWidth(v.Name), panelW))))

		// Stats
		lines = append(lines, "")
		lines = append(lines, styleTitle.Render("Items:    ")+styleInfo.Render(fmt.Sprintf("%d", v.ItemCount)))

		// Timestamps
		lines = append(lines, "")
		if v.CreatedAt != "" {
			ts := v.CreatedAt
			if len(ts) > 19 {
				ts = ts[:19]
			}
			lines = append(lines, styleTitle.Render("Created:  ")+styleDim.Render(ts))
		}
		if v.UpdatedAt != "" {
			ts := v.UpdatedAt
			if len(ts) > 19 {
				ts = ts[:19]
			}
			lines = append(lines, styleTitle.Render("Updated:  ")+styleDim.Render(ts))
		}

		// Key hints
		lines = append(lines, "")
		lines = append(lines, styleDim.Render("o:restore  n:new  D-D:delete"))
	}

	return lines
}

// rwPadRight pads s with spaces so its display width reaches targetW.
// Uses lipgloss.Width which handles ANSI escape sequences correctly.
// If s is already wider than targetW, it is returned unchanged.
func rwPadRight(s string, targetW int) string {
	w := lipgloss.Width(s)
	if w >= targetW {
		return s
	}
	return s + strings.Repeat(" ", targetW-w)
}

func (a *App) renderItem(item any) string {
	// Available width for content (minus 2 for prefix)
	w := a.width - 4
	if w < 40 {
		w = 40
	}

	switch v := item.(type) {
	case TabItem:
		// [icon] [title 50%]  [domain 25%]
		// Icon: ● (active, green), ⊕ (pinned, orange), ○ (normal, dim)
		var icon string
		if v.Pinned {
			icon = lipgloss.NewStyle().Foreground(colorOrange).Render("\u2295") + " " // ⊕
		} else if v.Active {
			icon = styleAccent.Render("\u25cf") + " " // ●
		} else {
			icon = styleDim.Render("\u25cb") + " " // ○
		}

		titleW := w * 50 / 100
		domainW := w * 25 / 100

		title := rwTruncate(v.Title, titleW, "...")
		domain := extractDomain(v.URL)
		domain = rwTruncate(domain, domainW, "...")

		titlePad := titleW - runewidth.StringWidth(title)
		if titlePad < 0 {
			titlePad = 0
		}
		domainPad := domainW - runewidth.StringWidth(domain)
		if domainPad < 0 {
			domainPad = 0
		}

		var flags string
		if v.Muted {
			flags = styleFlags.Render(" \U0001f507")
		}

		return icon + styleTitle.Render(title) + strings.Repeat(" ", titlePad) + "  " +
			styleURLDomain.Render(domain) + strings.Repeat(" ", domainPad) + flags

	case GroupItem:
		// [color dot] Title  collapsed/expanded  tabCount
		colorDot := groupColorDot(v.Color)
		state := styleAccent.Render("expanded")
		if v.Collapsed {
			state = styleDim.Render("collapsed")
		}
		return colorDot + " " + styleTitle.Render(v.Title) + "  " + state

	case SessionItem:
		icon := "\u25b8 " // ▸ collapsed
		if _, ok := a.expandedSessions[v.Name]; ok {
			icon = "\u25be " // ▾ expanded
		}
		name := rwTruncate(v.Name, 22, "...")
		stats := styleInfo.Render(fmt.Sprintf("%dt %dw %dg", v.TabCount, v.WindowCount, v.GroupCount))
		ts := ""
		if len(v.CreatedAt) >= 10 {
			ts = styleDim.Render(v.CreatedAt[:min(19, len(v.CreatedAt))])
		}
		return stylePurple.Render(icon) + stylePurple.Render(name) + "  " + stats + "  " + ts

	case CollectionItem:
		icon := "\u25b8 " // ▸ collapsed
		if _, ok := a.expandedCollections[v.Name]; ok {
			icon = "\u25be " // ▾ expanded
		}
		name := rwTruncate(v.Name, 22, "...")
		stats := styleInfo.Render(fmt.Sprintf("%d items", v.ItemCount))
		ts := ""
		if len(v.UpdatedAt) >= 10 {
			ts = styleDim.Render(v.UpdatedAt[:min(19, len(v.UpdatedAt))])
		}
		return stylePurple.Render(icon) + stylePurple.Render(name) + "  " + stats + "  " + ts

	case NestedTabItem:
		indent := "  "
		bullet := styleDim.Render("\u2022 ")
		titleW := w * 50 / 100
		domainW := w * 25 / 100
		title := rwTruncate(v.Title, titleW, "...")
		domain := extractDomain(v.URL)
		domain = rwTruncate(domain, domainW, "...")
		titlePad := titleW - runewidth.StringWidth(title)
		if titlePad < 0 {
			titlePad = 0
		}
		return indent + bullet + styleTitle.Render(title) + strings.Repeat(" ", titlePad) + "  " + styleURLDomain.Render(domain)

	case TargetItem:
		// browser name / label  channel
		def := ""
		if v.IsDefault {
			def = styleAccent.Render("*") + " "
		}
		browser := extractBrowserName(v.UserAgent)
		version := extractBrowserVersion(v.UserAgent)
		// Check for duplicate browser names across all targets
		displayName := browser
		if version != "" {
			vs := a.views[ViewTargets]
			dupeCount := 0
			for _, item := range vs.items {
				if t, ok := item.(TargetItem); ok {
					if extractBrowserName(t.UserAgent) == browser {
						dupeCount++
					}
				}
			}
			if dupeCount > 1 {
				displayName = browser + " " + version
			}
		}
		label := v.Label
		if label != "" {
			label = styleDim.Render("(") + styleTitle.Render(label) + styleDim.Render(")")
		}
		channel := styleDim.Render(v.Channel)
		return def + styleTitle.Render(displayName) + " " + label + "  " + channel

	case BookmarkItem:
		indent := strings.Repeat("  ", v.Depth)
		if v.IsFolder {
			icon := "\u25be " // ▾ expanded
			if a.collapsedFolders[v.ID] {
				icon = "\u25b8 " // ▸ collapsed
			}
			childCount := styleDim.Render(fmt.Sprintf(" (%d)", len(v.Children)))
			return indent + stylePurple.Render(icon) + stylePurple.Render(v.Title) + childCount
		}
		contentW := w - v.Depth*2 - 4 // available width after indent + bullet
		domainW := w * 25 / 100
		if domainW > 30 {
			domainW = 30
		}
		titleW := contentW - domainW - 2 // -2 for gap
		if titleW < 10 {
			titleW = 10
		}
		title := rwTruncate(v.Title, titleW, "...")
		domain := extractDomain(v.URL)
		domain = rwTruncate(domain, domainW, "...")
		titlePad := titleW - runewidth.StringWidth(title)
		if titlePad < 0 {
			titlePad = 0
		}
		bullet := styleDim.Render("\u2022 ") // bullet
		return indent + bullet + styleTitle.Render(title) + strings.Repeat(" ", titlePad) + "  " + styleURLDomain.Render(domain)

	case WorkspaceItem:
		name := rwTruncate(v.Name, 24, "...")
		stats := styleInfo.Render(fmt.Sprintf("%ds %dc", v.SessionCount, v.CollectionCount))
		ts := ""
		if len(v.UpdatedAt) >= 10 {
			ts = styleDim.Render(v.UpdatedAt[:min(19, len(v.UpdatedAt))])
		}
		return stylePurple.Render(name) + "  " + stats + "  " + ts

	case HistoryItem:
		titleW := w * 50 / 100
		domainW := w * 25 / 100
		title := rwTruncate(v.Title, titleW, "...")
		domain := extractDomain(v.URL)
		domain = rwTruncate(domain, domainW, "...")
		titlePad := titleW - runewidth.StringWidth(title)
		if titlePad < 0 {
			titlePad = 0
		}
		visits := styleDim.Render(fmt.Sprintf("(%d)", v.VisitCount))
		return styleTitle.Render(title) + strings.Repeat(" ", titlePad) + "  " +
			styleURLDomain.Render(domain) + "  " + visits

	case SyncStatusItem:
		var statusText string
		if v.Enabled {
			statusText = styleAccent.Render("enabled")
		} else {
			statusText = styleDim.Render("disabled")
		}
		line := styleTitle.Render("Sync: ") + statusText
		line += "  " + styleTitle.Render("Dir: ") + styleDim.Render(v.SyncDir)
		if v.LastSync != "" {
			line += "  " + styleTitle.Render("Last: ") + styleDim.Render(v.LastSync[:min(19, len(v.LastSync))])
		}
		line += "  " + styleTitle.Render("Pending: ") + styleInfo.Render(fmt.Sprintf("%d", v.PendingChanges))
		if len(v.Conflicts) > 0 {
			line += "  " + styleDanger.Render(fmt.Sprintf("Conflicts: %d", len(v.Conflicts)))
		}
		return line

	case SearchResultItem:
		// [kind icon] title  domain  (score)
		kindIcons := map[string]string{
			"tab": "\u25cf", "session": "\u25b8", "collection": "\u25a0",
			"bookmark": "\u2606", "workspace": "\u25c6",
		}
		icon := kindIcons[v.Kind]
		if icon == "" {
			icon = "\u00b7"
		}
		kindStyle := lipgloss.NewStyle().Foreground(colorPrimary)
		titleW := w * 50 / 100
		domainW := w * 25 / 100
		title := rwTruncate(v.Title, titleW, "...")
		domain := extractDomain(v.URL)
		domain = rwTruncate(domain, domainW, "...")
		titlePad := titleW - runewidth.StringWidth(title)
		if titlePad < 0 {
			titlePad = 0
		}
		score := styleDim.Render(fmt.Sprintf("%.0f%%", v.Score*100))
		kindLabel := styleDim.Render("[" + v.Kind + "]")
		return kindStyle.Render(icon) + " " + styleTitle.Render(title) + strings.Repeat(" ", titlePad) + "  " +
			styleURLDomain.Render(domain) + "  " + score + " " + kindLabel

	case SavedSearchItem:
		// [icon] name  "query text"  date
		icon := stylePurple.Render("\u2605 ") // ★
		name := rwTruncate(v.Name, 24, "...")
		query := styleDim.Render("\"" + rwTruncate(v.QueryText, 30, "...") + "\"")
		ts := ""
		if len(v.CreatedAt) >= 10 {
			ts = styleDim.Render(v.CreatedAt[:min(10, len(v.CreatedAt))])
		}
		return icon + stylePurple.Render(name) + "  " + query + "  " + ts

	case DownloadItem:
		// filename  state  size
		titleW := w * 50 / 100
		filename := rwTruncate(v.Filename, titleW, "...")
		titlePad := titleW - runewidth.StringWidth(filename)
		if titlePad < 0 {
			titlePad = 0
		}
		var stateStyle lipgloss.Style
		switch v.State {
		case "complete":
			stateStyle = lipgloss.NewStyle().Foreground(colorAccent)
		case "in_progress":
			stateStyle = lipgloss.NewStyle().Foreground(colorPrimary)
		default:
			stateStyle = lipgloss.NewStyle().Foreground(colorWarn)
		}
		size := styleDim.Render(formatBytes(v.TotalBytes))
		return styleTitle.Render(filename) + strings.Repeat(" ", titlePad) + "  " +
			stateStyle.Render(v.State) + "  " + size
	}
	return fmt.Sprintf("%v", item)
}

func (a *App) renderStatusBar() string {
	// Priority: Error > ConfirmHint > Toast > normal bar
	if a.errorMsg != "" {
		return styleError.Render("ERROR: " + a.errorMsg + "  (Esc to clear)")
	}
	if a.confirmHint != "" {
		return styleConfirm.Render(a.confirmHint)
	}

	// Left: view name + item count + selected count
	vs := a.currentView()
	leftParts := []string{styleInfo.Render(a.view.String())}
	leftParts = append(leftParts, styleDim.Render(fmt.Sprintf("%d items", vs.visibleCount())))
	selCount := len(vs.selected)
	if selCount > 0 {
		leftParts = append(leftParts, styleWarnText(fmt.Sprintf("%d selected", selCount)))
	}
	left := strings.Join(leftParts, styleDim.Render(" \u2022 "))

	// Center: toast
	center := ""
	if a.toast != "" {
		center = styleToast.Render(a.toast)
	}

	// Right: key hints summary
	right := ""
	bindings := bindingsForView(a.view)
	if len(bindings) > 0 {
		var hints []string
		maxHints := 4
		for i, b := range bindings {
			if i >= maxHints {
				break
			}
			hints = append(hints, styleDim.Render(b.Key)+styleFlags.Render(":"+b.Desc))
		}
		right = strings.Join(hints, " ")
	}

	// Layout: left ... center ... right
	leftW := lipgloss.Width(left)
	centerW := lipgloss.Width(center)
	rightW := lipgloss.Width(right)

	if center != "" {
		gapLC := (a.width-leftW-centerW-rightW)/2 - 1
		if gapLC < 2 {
			gapLC = 2
		}
		gapCR := a.width - leftW - gapLC - centerW - rightW
		if gapCR < 1 {
			gapCR = 1
		}
		return left + strings.Repeat(" ", gapLC) + center + strings.Repeat(" ", gapCR) + right
	}

	gap := a.width - leftW - rightW
	if gap < 0 {
		gap = 0
	}
	return left + strings.Repeat(" ", gap) + right
}

func (a *App) renderHelp() string {
	var b strings.Builder
	b.WriteString(styleHeader.Render("CTM Help") + "\n\n")

	b.WriteString(styleHeader.Render("Global") + "\n")
	for _, kb := range globalBindings() {
		b.WriteString(fmt.Sprintf("  %-12s %s\n", kb.Key, kb.Desc))
	}

	b.WriteString("\n" + styleHeader.Render("Navigation") + "\n")
	for _, kb := range navigationBindings() {
		b.WriteString(fmt.Sprintf("  %-12s %s\n", kb.Key, kb.Desc))
	}

	for _, v := range []ViewType{ViewTargets, ViewTabs, ViewGroups, ViewSessions, ViewCollections, ViewBookmarks, ViewWorkspaces, ViewSync, ViewHistory, ViewSearch, ViewDownloads} {
		bindings := bindingsForView(v)
		if len(bindings) == 0 {
			continue
		}
		b.WriteString("\n" + styleHeader.Render(v.String()) + "\n")
		for _, kb := range bindings {
			b.WriteString(fmt.Sprintf("  %-12s %s\n", kb.Key, kb.Desc))
		}
	}

	b.WriteString("\n" + styleDim.Render("Press Esc/q/? to close"))
	return b.String()
}

// --- Helpers ---

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max < 4 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

// rwTruncate truncates a string to fit within maxWidth display columns,
// handling CJK wide characters correctly. Uses go-runewidth.
func rwTruncate(s string, maxWidth int, tail string) string {
	if runewidth.StringWidth(s) <= maxWidth {
		return s
	}
	return runewidth.Truncate(s, maxWidth, tail)
}

// extractDomain extracts just the domain from a URL (no path, no scheme).
// e.g. "https://github.com/user/repo" -> "github.com"
func extractDomain(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		// Fallback to simple extraction
		return extractHost(rawURL)
	}
	return u.Hostname()
}

// extractBrowserName extracts a human-readable browser name from a User-Agent string.
func extractBrowserName(ua string) string {
	if ua == "" {
		return "Browser"
	}
	uaLower := strings.ToLower(ua)
	switch {
	case strings.Contains(uaLower, "edg/") || strings.Contains(uaLower, "edge/"):
		return "Edge"
	case strings.Contains(uaLower, "arc/") || strings.Contains(uaLower, "arc "):
		return "Arc"
	case strings.Contains(uaLower, "brave/"):
		return "Brave"
	case strings.Contains(uaLower, "vivaldi/"):
		return "Vivaldi"
	case strings.Contains(uaLower, "opr/") || strings.Contains(uaLower, "opera"):
		return "Opera"
	case strings.Contains(uaLower, "chrome/"):
		// Check for Chrome Beta
		if strings.Contains(ua, "Chrome Beta") || strings.Contains(uaLower, "chrome beta") {
			return "Chrome Beta"
		}
		return "Chrome"
	case strings.Contains(uaLower, "safari/") && !strings.Contains(uaLower, "chrome"):
		return "Safari"
	case strings.Contains(uaLower, "firefox/"):
		return "Firefox"
	}
	return "Browser"
}

// extractBrowserVersion extracts the browser major version from a User-Agent string.
// e.g. "Mozilla/5.0 ... Chrome/145.0.0.0 ..." -> "145"
func extractBrowserVersion(ua string) string {
	if ua == "" {
		return ""
	}
	uaLower := strings.ToLower(ua)

	// Try to find the relevant version token based on browser type
	var token string
	switch {
	case strings.Contains(uaLower, "edg/"):
		token = "Edg/"
	case strings.Contains(uaLower, "chrome/"):
		token = "Chrome/"
	case strings.Contains(uaLower, "firefox/"):
		token = "Firefox/"
	case strings.Contains(uaLower, "safari/") && !strings.Contains(uaLower, "chrome"):
		token = "Version/"
	default:
		return ""
	}

	idx := strings.Index(ua, token)
	if idx < 0 {
		// Case-insensitive fallback
		idx = strings.Index(uaLower, strings.ToLower(token))
		if idx < 0 {
			return ""
		}
	}
	rest := ua[idx+len(token):]
	// Extract version up to first dot or space
	end := strings.IndexAny(rest, ". ")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

// groupColorDot returns a styled color indicator for Chrome tab groups.
func groupColorDot(color string) string {
	colorMap := map[string]lipgloss.Color{
		"grey":   lipgloss.Color("#9AA0A6"),
		"blue":   lipgloss.Color("#8AB4F8"),
		"red":    lipgloss.Color("#F28B82"),
		"yellow": lipgloss.Color("#FDD663"),
		"green":  lipgloss.Color("#81C995"),
		"pink":   lipgloss.Color("#FF8BCB"),
		"purple": lipgloss.Color("#D7AEFB"),
		"cyan":   lipgloss.Color("#78D9EC"),
		"orange": lipgloss.Color("#FCAD70"),
	}
	c, ok := colorMap[strings.ToLower(color)]
	if !ok {
		c = lipgloss.Color("#9AA0A6")
	}
	return lipgloss.NewStyle().Foreground(c).Render("\u25cf")
}

// foldAllBookmarks recursively marks all folders as collapsed.
func (a *App) foldAllBookmarks(item BookmarkItem) {
	isFolder := len(item.Children) > 0 || item.URL == ""
	if isFolder && len(item.Children) > 0 {
		a.collapsedFolders[item.ID] = true
		for _, child := range item.Children {
			a.foldAllBookmarks(child)
		}
	}
}

// renderKittyImage renders a base64 PNG data URL using Kitty graphics protocol.
// Returns lines containing the escape sequences that Kitty will render as an image.
func (a *App) renderKittyImage(dataURL string, maxW, maxH int) []string {
	// Strip "data:image/png;base64," prefix
	idx := strings.Index(dataURL, ",")
	if idx < 0 {
		return []string{styleDim.Render("(invalid image data)")}
	}
	b64Data := dataURL[idx+1:]

	// Decode to get raw PNG bytes
	imgBytes, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return []string{styleDim.Render("(decode error)")}
	}

	// Re-encode to base64 for Kitty protocol (it expects raw base64, no data URL prefix)
	kittyB64 := base64.StdEncoding.EncodeToString(imgBytes)

	// Kitty graphics protocol: split into 4096-byte chunks
	// First chunk: a=T (transmit and display), f=100 (PNG), c=columns, r=rows
	cols := maxW
	rows := maxH
	if rows < 1 {
		rows = 1
	}

	var lines []string
	// Single escape sequence that places the image
	// \033_G is the Kitty graphics start, a=T means transmit+display,
	// f=100 means PNG, C=1 means do not move cursor, q=2 means suppress responses
	chunkSize := 4096
	for i := 0; i < len(kittyB64); i += chunkSize {
		end := i + chunkSize
		if end > len(kittyB64) {
			end = len(kittyB64)
		}
		chunk := kittyB64[i:end]
		more := 1
		if end >= len(kittyB64) {
			more = 0
		}
		if i == 0 {
			// First chunk with metadata
			lines = append(lines, fmt.Sprintf("\033_Ga=T,f=100,c=%d,r=%d,q=2,m=%d;%s\033\\", cols, rows, more, chunk))
		} else {
			// Continuation chunk
			lines = append(lines, fmt.Sprintf("\033_Gm=%d;%s\033\\", more, chunk))
		}
	}

	if len(lines) == 0 {
		return []string{styleDim.Render("(empty image)")}
	}

	return lines
}

// reflattenBookmarks re-flattens the bookmark tree using the current collapse state.
func (a *App) reflattenBookmarks() {
	vs := a.views[ViewBookmarks]
	flat := flattenBookmarkTreeWithCollapse(a.bookmarkTree, 0, a.collapsedFolders)
	vs.items = make([]any, len(flat))
	for i, b := range flat {
		vs.items[i] = b
	}
	vs.itemCount = len(flat)
	vs.clampCursor()
}

// styleWarnText renders text in warning color.
func styleWarnText(s string) string {
	return lipgloss.NewStyle().Foreground(colorWarn).Render(s)
}

// flattenBookmarkTree converts a nested BookmarkItem tree into a flat list for TUI display,
// preserving depth information for indented rendering.
func flattenBookmarkTree(tree []BookmarkItem, depth int) []BookmarkItem {
	return flattenBookmarkTreeWithCollapse(tree, depth, nil)
}

// flattenBookmarkTreeWithCollapse flattens the tree, skipping children of collapsed folders.
// Root nodes with empty titles (Chrome's invisible root "0") are skipped — their children
// are promoted to the same depth level.
func flattenBookmarkTreeWithCollapse(tree []BookmarkItem, depth int, collapsed map[string]bool) []BookmarkItem {
	var result []BookmarkItem
	for _, item := range tree {
		isFolder := len(item.Children) > 0 || item.URL == ""
		// Skip invisible root node (Chrome's root "0" has empty title)
		skipSelf := depth == 0 && item.Title == "" && isFolder
		if !skipSelf {
			flat := BookmarkItem{
				ID:       item.ID,
				Title:    item.Title,
				URL:      item.URL,
				ParentID: item.ParentID,
				Children: item.Children, // keep Children reference for child count
				Depth:    depth,
				IsFolder: isFolder,
			}
			result = append(result, flat)
		}
		// Recurse into children if not collapsed (skipped nodes always recurse)
		childDepth := depth + 1
		if skipSelf {
			childDepth = depth // promoted children keep same depth
		}
		if len(item.Children) > 0 && (skipSelf || collapsed == nil || !collapsed[item.ID]) {
			result = append(result, flattenBookmarkTreeWithCollapse(item.Children, childDepth, collapsed)...)
		}
	}
	return result
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func extractHost(rawURL string) string {
	// Simple host extraction
	u := rawURL
	if idx := strings.Index(u, "://"); idx >= 0 {
		u = u[idx+3:]
	}
	if idx := strings.Index(u, "/"); idx >= 0 {
		u = u[:idx]
	}
	return u
}
