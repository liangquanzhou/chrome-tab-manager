package daemon

import (
	"fmt"

	"ctm/internal/protocol"
	ctmsync "ctm/internal/sync"
)

func (h *Hub) handleSyncAction(incoming *incomingMessage) {
	switch incoming.msg.Action {
	case "sync.status":
		h.handleSyncStatus(incoming)
	case "sync.repair":
		h.handleSyncRepair(incoming)
	default:
		err := &UnknownActionError{Action: incoming.msg.Action}
		sendError(incoming.writer, incoming.msg.ID, daemonErrorCode(err), err.Error())
	}
}

func (h *Hub) handleSyncStatus(incoming *incomingMessage) {
	engine := ctmsync.NewSyncEngine(h.syncLocalDir(), h.syncCloudDir)

	status, err := engine.Status()
	if err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("sync status: %s", err))
		return
	}

	sendResponse(incoming.writer, incoming.msg.ID, status)
}

func (h *Hub) handleSyncRepair(incoming *incomingMessage) {
	engine := ctmsync.NewSyncEngine(h.syncLocalDir(), h.syncCloudDir)

	result, err := engine.Repair()
	if err != nil {
		sendError(incoming.writer, incoming.msg.ID, protocol.ErrInvalidPayload,
			fmt.Sprintf("sync repair: %s", err))
		return
	}

	sendResponse(incoming.writer, incoming.msg.ID, result)
}

// syncLocalDir returns the base config dir that contains sessions/, collections/, workspaces/.
// The sync engine will copy these subdirectories to the cloud dir.
func (h *Hub) syncLocalDir() string {
	// The sessionsDir is like ~/.config/ctm/sessions,
	// so the parent is ~/.config/ctm/ which is the local dir.
	// We use the parent of sessionsDir.
	if h.sessionsDir != "" {
		parent := h.sessionsDir[:len(h.sessionsDir)-len("/sessions")]
		if parent != "" {
			return parent
		}
	}
	return h.sessionsDir
}
