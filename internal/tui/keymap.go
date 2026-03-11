package tui

type KeyBinding struct {
	Key  string
	Desc string
}

type ViewBindings struct {
	View     ViewType
	Bindings []KeyBinding
}

func globalBindings() []KeyBinding {
	return []KeyBinding{
		{"q", "quit"},
		{"Esc", "cancel/close"},
		{"?", "help"},
		{":", "command"},
		{"/", "filter"},
		{"r", "refresh"},
		{"Tab", "next view"},
		{"1-9", "switch view"},
	}
}

func navigationBindings() []KeyBinding {
	return []KeyBinding{
		{"j/↓", "down"},
		{"k/↑", "up"},
		{"gg", "top"},
		{"G", "bottom"},
		{"Space", "select"},
		{"Enter", "action"},
	}
}

func bindingsForView(v ViewType) []KeyBinding {
	switch v {
	case ViewTabs:
		return []KeyBinding{
			{"Enter", "activate"},
			{"x", "close"},
			{"m", "mute/unmute"},
			{"p", "pin/unpin"},
			{"v", "toggle preview"},
			{"s", "screenshot (open)"},
			{"P", "w3m preview"},
			{"M", "move to window"},
			{"A", "add to collection"},
			{"n", "group selected"},
			{"Space", "select"},
			{"u", "clear selection"},
			{"y·", "yank/copy"},
			{"z·", "filter by"},
		}
	case ViewGroups:
		return []KeyBinding{
			{"Enter", "expand/collapse"},
			{"D D", "delete"},
		}
	case ViewSessions:
		return []KeyBinding{
			{"Enter", "preview"},
			{"o", "restore"},
			{"n", "save new"},
			{"x x", "delete"},
		}
	case ViewCollections:
		return []KeyBinding{
			{"Enter", "expand"},
			{"o", "restore"},
			{"n", "create new"},
			{"e", "rename"},
			{"x", "remove item"},
			{"x x", "delete collection"},
			{"J/K", "move item"},
			{"y·", "yank/copy"},
		}
	case ViewTargets:
		return []KeyBinding{
			{"Enter", "activate"},
			{"d", "set default"},
			{"c", "clear default"},
			{"l", "set label"},
		}
	case ViewBookmarks:
		return []KeyBinding{
			{"Enter", "open/toggle"},
			{"a", "add bookmark"},
			{"E", "export"},
			{"l/→", "expand folder"},
			{"h/←", "collapse/parent"},
			{"D D", "delete"},
			{"/", "search"},
			{"zM", "fold all"},
			{"zR", "unfold all"},
			{"r", "re-mirror"},
		}
	case ViewWorkspaces:
		return []KeyBinding{
			{"Enter", "inspect"},
			{"o", "switch"},
			{"n", "create new"},
			{"e", "edit name"},
			{"D D", "delete"},
		}
	case ViewSync:
		return []KeyBinding{
			{"r", "refresh status"},
			{"R", "repair"},
		}
	case ViewHistory:
		return []KeyBinding{
			{"Enter", "open in browser"},
			{"/", "search"},
			{"D D", "delete from history"},
		}
	case ViewSearch:
		return []KeyBinding{
			{"/", "search"},
			{"Enter", "open result"},
			{"n", "save search"},
			{"D D", "delete saved"},
		}
	case ViewDownloads:
		return []KeyBinding{
			{"x", "cancel"},
		}
	}
	return nil
}
