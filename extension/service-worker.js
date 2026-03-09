// CTM Chrome Extension — Service Worker
//
// Bridges Chrome browser APIs to the CTM daemon via Native Messaging.
//
// Architecture:
//   service-worker.js <-> Native Messaging (Port) <-> ctm nm-shim <-> ctm daemon
//
// Chrome handles the 4-byte LE framing on the NM port automatically.
// We send/receive plain JSON objects through the port.

"use strict";

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const NM_HOST = "com.ctm.native_host";
const PROTOCOL_VERSION = 1;

// Reconnection backoff parameters (milliseconds)
const RECONNECT_INITIAL_MS = 1000;
const RECONNECT_MAX_MS = 30000;
const RECONNECT_MULTIPLIER = 2;

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

/** @type {chrome.runtime.Port|null} */
let port = null;

/** @type {string|null} Target ID assigned by daemon after hello handshake */
let targetId = null;

/** Current reconnection delay in ms */
let reconnectDelay = RECONNECT_INITIAL_MS;

/** Handle for any pending reconnect timer */
let reconnectTimer = null;

/** True while a connect attempt is in progress */
let connecting = false;

/** @type {Map<string, {resolve: Function, reject: Function}>} Pending outgoing requests */
const pendingRequests = new Map();

// ---------------------------------------------------------------------------
// UUID generation
// ---------------------------------------------------------------------------

/**
 * Generate a unique message ID using crypto.randomUUID().
 * @returns {string}
 */
function makeId() {
  return crypto.randomUUID();
}

// ---------------------------------------------------------------------------
// Logging helpers
// ---------------------------------------------------------------------------

function log(msg, ...args) {
  console.log(`[CTM ${timestamp()}] ${msg}`, ...args);
}

function warn(msg, ...args) {
  console.warn(`[CTM ${timestamp()}] ${msg}`, ...args);
}

function timestamp() {
  const d = new Date();
  return (
    String(d.getHours()).padStart(2, "0") +
    ":" +
    String(d.getMinutes()).padStart(2, "0") +
    ":" +
    String(d.getSeconds()).padStart(2, "0")
  );
}

// ---------------------------------------------------------------------------
// Native Messaging connection
// ---------------------------------------------------------------------------

/**
 * Establish a Native Messaging connection and register listeners.
 * No-op if already connected or if a connect attempt is in flight.
 */
function connect() {
  if (port !== null || connecting) {
    return;
  }
  connecting = true;

  try {
    port = chrome.runtime.connectNative(NM_HOST);
  } catch (err) {
    warn("connectNative failed:", err.message);
    port = null;
    connecting = false;
    scheduleReconnect();
    return;
  }

  port.onMessage.addListener(onNativeMessage);
  port.onDisconnect.addListener(onDisconnect);

  connecting = false;

  // Reset backoff on successful connection
  reconnectDelay = RECONNECT_INITIAL_MS;

  log("Native Messaging port opened");
  sendHello();
}

/**
 * Handle native port disconnect.
 * Chrome fires this when the NM host process exits, crashes, or cannot start.
 */
function onDisconnect() {
  const lastError = chrome.runtime.lastError;
  if (lastError) {
    warn("Native port disconnected:", lastError.message);
  } else {
    warn("Native port disconnected");
  }

  port = null;
  targetId = null;
  scheduleReconnect();
}

/**
 * Schedule a reconnection attempt with exponential backoff.
 */
function scheduleReconnect() {
  if (reconnectTimer !== null) {
    return; // already scheduled
  }

  log(`Reconnecting in ${reconnectDelay}ms`);
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null;
    connect();
  }, reconnectDelay);

  // Increase delay for next attempt (capped)
  reconnectDelay = Math.min(
    reconnectDelay * RECONNECT_MULTIPLIER,
    RECONNECT_MAX_MS
  );
}

// ---------------------------------------------------------------------------
// Hello handshake
// ---------------------------------------------------------------------------

/**
 * Send a hello message to register this extension as a target.
 * Uses pendingRequests so the response is matched by ID (daemon responses
 * do not include an action field).
 */
function sendHello() {
  const id = makeId();

  pendingRequests.set(id, {
    resolve: (payload) => {
      if (payload && payload.targetId) {
        targetId = payload.targetId;
        log("Registered as target:", targetId);
        refreshCollectionsMenu();
      } else {
        warn("Hello response missing targetId");
      }
    },
    reject: (err) => {
      warn("Hello handshake failed:", err.message);
    },
  });

  sendRaw({
    id,
    protocol_version: PROTOCOL_VERSION,
    type: "hello",
    action: "hello",
    payload: {
      channel: "stable",
      extensionId: chrome.runtime.id,
      instanceId: makeId(),
      userAgent: navigator.userAgent,
      capabilities: ["tabs", "groups", "bookmarks", "history", "downloads", "windows", "events"],
      min_supported: PROTOCOL_VERSION,
    },
  });

  log("Hello sent");
}

// ---------------------------------------------------------------------------
// Incoming message handling
// ---------------------------------------------------------------------------

/**
 * Handle a message received from the native port.
 * @param {object} msg - Parsed JSON message from NM host
 */
function onNativeMessage(msg) {
  if (!msg || typeof msg !== "object") {
    warn("Received invalid message (not an object)");
    return;
  }

  // Resolve pending outgoing requests (including hello handshake)
  if (msg.type === "response" && pendingRequests.has(msg.id)) {
    const { resolve } = pendingRequests.get(msg.id);
    pendingRequests.delete(msg.id);
    resolve(msg.payload);
    return;
  }
  if (msg.type === "error" && pendingRequests.has(msg.id)) {
    const { reject } = pendingRequests.get(msg.id);
    pendingRequests.delete(msg.id);
    reject(new Error(msg.error?.message || "Unknown error"));
    return;
  }

  // Incoming request from daemon
  if (msg.type === "request") {
    handleRequest(msg);
    return;
  }

  // Ignore other message types (responses to our events, etc.)
  log("Ignoring message type:", msg.type, "action:", msg.action);
}

// ---------------------------------------------------------------------------
// Request dispatch
// ---------------------------------------------------------------------------

/**
 * Handle an incoming request: dispatch to the appropriate handler,
 * then send a response or error back.
 * @param {object} msg - Request message
 */
async function handleRequest(msg) {
  const id = msg.id;
  const action = msg.action;
  const payload = msg.payload || {};

  try {
    const result = await dispatch(action, payload);
    sendResponse(id, action, result);
  } catch (err) {
    warn(`Action ${action} failed:`, err.message);
    sendError(id, action, "EXTENSION_ERROR", err.message);
  }
}

/**
 * Route an action to its Chrome API handler.
 * @param {string} action - The action name (e.g. "tabs.list")
 * @param {object} payload - The request payload
 * @returns {Promise<object>} - The response payload
 */
async function dispatch(action, payload) {
  switch (action) {
    case "tabs.list":
      return handleTabsList();
    case "tabs.open":
      return handleTabsOpen(payload);
    case "tabs.close":
      return handleTabsClose(payload);
    case "tabs.activate":
      return handleTabsActivate(payload);
    case "tabs.update":
      return handleTabsUpdate(payload);
    case "groups.list":
      return handleGroupsList();
    case "groups.create":
      return handleGroupsCreate(payload);
    case "groups.update":
      return handleGroupsUpdate(payload);
    case "groups.delete":
      return handleGroupsDelete(payload);
    case "bookmarks.tree":
      return handleBookmarksTree();
    case "bookmarks.search":
      return handleBookmarksSearch(payload);
    case "bookmarks.get":
      return handleBookmarksGet(payload);
    case "tabs.getText":
      return handleTabsGetText(payload);
    case "tabs.mute":
      return handleTabsMute(payload);
    case "tabs.pin":
      return handleTabsPin(payload);
    case "tabs.move":
      return handleTabsMove(payload);
    case "bookmarks.move":
      return handleBookmarksMove(payload);
    case "bookmarks.remove":
      return handleBookmarksRemove(payload);
    case "bookmarks.create":
      return handleBookmarksCreate(payload);
    case "bookmarks.update":
      return handleBookmarksUpdate(payload);
    case "windows.list":
      return handleWindowsList();
    case "windows.create":
      return handleWindowsCreate(payload);
    case "windows.close":
      return handleWindowsClose(payload);
    case "windows.focus":
      return handleWindowsFocus();
    case "history.search":
      return handleHistorySearch(payload);
    case "history.delete":
      return handleHistoryDelete(payload);
    case "downloads.list":
      return handleDownloadsList(payload);
    case "downloads.cancel":
      return handleDownloadsCancel(payload);
    case "tabs.capture":
      return handleTabsCapture(payload);
    default:
      throw new Error(`Unknown action: ${action}`);
  }
}

// ---------------------------------------------------------------------------
// Chrome API handlers — Tabs
// ---------------------------------------------------------------------------

/**
 * tabs.list: Query all tabs across all windows.
 * @returns {Promise<{tabs: object[]}>}
 */
async function handleTabsList() {
  const tabs = await chrome.tabs.query({});
  return {
    tabs: tabs.map((t) => ({
      id: t.id,
      windowId: t.windowId,
      index: t.index,
      title: t.title || "",
      url: t.url || "",
      active: t.active,
      pinned: t.pinned,
      muted: !!(t.mutedInfo && t.mutedInfo.muted),
      groupId: t.groupId,
      favIconUrl: t.favIconUrl || "",
    })),
  };
}

/**
 * tabs.open: Open a new tab or reuse an existing one (deduplication).
 * @param {object} payload
 * @param {string} payload.url - URL to open
 * @param {boolean} [payload.active=true] - Whether to make the tab active
 * @param {boolean} [payload.focus=false] - Whether to focus the window
 * @param {boolean} [payload.deduplicate=false] - Reuse existing tab with same URL
 * @param {number} [payload.windowId] - Target window ID
 * @returns {Promise<{tabId: number, windowId: number, reused: boolean}>}
 */
async function handleTabsOpen(payload) {
  const url = payload.url;
  if (!url) {
    throw new Error("tabs.open: url is required");
  }

  const active = payload.active !== false;
  const focus = payload.focus === true;
  const deduplicate = payload.deduplicate === true;

  // Deduplication: check for an existing tab with the same URL
  if (deduplicate) {
    const existing = await chrome.tabs.query({ url });
    if (existing.length > 0) {
      const tab = existing[0];
      await chrome.tabs.update(tab.id, { active: true });
      if (focus) {
        await chrome.windows.update(tab.windowId, { focused: true });
      }
      return { tabId: tab.id, windowId: tab.windowId, reused: true };
    }
  }

  // Create new tab
  const createProps = { url, active };
  if (payload.windowId !== undefined) {
    createProps.windowId = payload.windowId;
  }
  const tab = await chrome.tabs.create(createProps);

  if (focus && tab.windowId) {
    await chrome.windows.update(tab.windowId, { focused: true });
  }

  return { tabId: tab.id, windowId: tab.windowId, reused: false };
}

/**
 * tabs.close: Close a single tab by ID.
 * @param {object} payload
 * @param {number} payload.tabId - Tab ID to close
 * @returns {Promise<{}>}
 */
async function handleTabsClose(payload) {
  if (payload.tabId === undefined) {
    throw new Error("tabs.close: tabId is required");
  }
  await chrome.tabs.remove(payload.tabId);
  return {};
}

/**
 * tabs.activate: Activate a tab and optionally focus its window.
 * @param {object} payload
 * @param {number} payload.tabId - Tab ID to activate
 * @param {boolean} [payload.focus=false] - Whether to focus the window
 * @returns {Promise<{}>}
 */
async function handleTabsActivate(payload) {
  if (payload.tabId === undefined) {
    throw new Error("tabs.activate: tabId is required");
  }
  const tab = await chrome.tabs.update(payload.tabId, { active: true });
  if (payload.focus === true && tab.windowId) {
    await chrome.windows.update(tab.windowId, { focused: true });
  }
  return {};
}

/**
 * tabs.update: Update tab properties (currently: pinned state).
 * @param {object} payload
 * @param {number} payload.tabId - Tab ID to update
 * @param {boolean} [payload.pinned] - New pinned state
 * @returns {Promise<{}>}
 */
async function handleTabsUpdate(payload) {
  if (payload.tabId === undefined) {
    throw new Error("tabs.update: tabId is required");
  }
  const updateProps = {};
  if (payload.pinned !== undefined) {
    updateProps.pinned = payload.pinned;
  }
  await chrome.tabs.update(payload.tabId, updateProps);
  return {};
}

// ---------------------------------------------------------------------------
// Chrome API handlers — Groups
// ---------------------------------------------------------------------------

/**
 * groups.list: Query all tab groups.
 * @returns {Promise<{groups: object[]}>}
 */
async function handleGroupsList() {
  const groups = await chrome.tabGroups.query({});
  return {
    groups: groups.map((g) => ({
      id: g.id,
      title: g.title || "",
      color: g.color || "",
      collapsed: g.collapsed,
      windowId: g.windowId,
    })),
  };
}

/**
 * groups.create: Group tabs together and set title/color.
 * @param {object} payload
 * @param {number[]} payload.tabIds - Tab IDs to group
 * @param {string} [payload.title] - Group title
 * @param {string} [payload.color] - Group color
 * @returns {Promise<{groupId: number}>}
 */
async function handleGroupsCreate(payload) {
  if (!payload.tabIds || payload.tabIds.length === 0) {
    throw new Error("groups.create: tabIds is required");
  }
  const groupId = await chrome.tabs.group({ tabIds: payload.tabIds });

  const updateProps = {};
  if (payload.title !== undefined) {
    updateProps.title = payload.title;
  }
  if (payload.color !== undefined) {
    updateProps.color = payload.color;
  }
  if (Object.keys(updateProps).length > 0) {
    await chrome.tabGroups.update(groupId, updateProps);
  }

  return { groupId };
}

/**
 * groups.update: Update group properties (title, color, collapsed).
 * @param {object} payload
 * @param {number} payload.groupId - Group ID to update
 * @param {string} [payload.title] - New title
 * @param {string} [payload.color] - New color
 * @param {boolean} [payload.collapsed] - New collapsed state
 * @returns {Promise<{}>}
 */
async function handleGroupsUpdate(payload) {
  if (payload.groupId === undefined) {
    throw new Error("groups.update: groupId is required");
  }
  const updateProps = {};
  if (payload.title !== undefined) {
    updateProps.title = payload.title;
  }
  if (payload.color !== undefined) {
    updateProps.color = payload.color;
  }
  if (payload.collapsed !== undefined) {
    updateProps.collapsed = payload.collapsed;
  }
  await chrome.tabGroups.update(payload.groupId, updateProps);
  return {};
}

/**
 * groups.delete: Ungroup all tabs in a group (effectively deleting the group).
 * @param {object} payload
 * @param {number} payload.groupId - Group ID to delete
 * @returns {Promise<{}>}
 */
async function handleGroupsDelete(payload) {
  if (payload.groupId === undefined) {
    throw new Error("groups.delete: groupId is required");
  }
  const tabs = await chrome.tabs.query({ groupId: payload.groupId });
  if (tabs.length > 0) {
    const tabIds = tabs.map((t) => t.id);
    await chrome.tabs.ungroup(tabIds);
  }
  return {};
}

// ---------------------------------------------------------------------------
// Chrome API handlers — Bookmarks
// ---------------------------------------------------------------------------

/**
 * bookmarks.tree: Get the full bookmarks tree.
 * @returns {Promise<{tree: object[]}>}
 */
async function handleBookmarksTree() {
  const tree = await chrome.bookmarks.getTree();
  return { tree };
}

/**
 * bookmarks.search: Search bookmarks by query string.
 * @param {object} payload
 * @param {string} payload.query - Search query
 * @returns {Promise<{bookmarks: object[]}>}
 */
async function handleBookmarksSearch(payload) {
  if (!payload.query) {
    throw new Error("bookmarks.search: query is required");
  }
  const results = await chrome.bookmarks.search(payload.query);
  return {
    bookmarks: results.map((b) => ({
      id: b.id,
      title: b.title || "",
      url: b.url || "",
      parentId: b.parentId || "",
      dateAdded: b.dateAdded,
    })),
  };
}

/**
 * bookmarks.get: Get a single bookmark by ID.
 * @param {object} payload
 * @param {string} payload.id - Bookmark ID
 * @returns {Promise<{bookmark: object}>}
 */
async function handleBookmarksGet(payload) {
  if (!payload.id) {
    throw new Error("bookmarks.get: id is required");
  }
  const results = await chrome.bookmarks.get([payload.id]);
  if (results.length === 0) {
    throw new Error(`Bookmark not found: ${payload.id}`);
  }
  const b = results[0];
  return {
    bookmark: {
      id: b.id,
      title: b.title || "",
      url: b.url || "",
      parentId: b.parentId || "",
      dateAdded: b.dateAdded,
    },
  };
}

// ---------------------------------------------------------------------------
// Chrome API handlers — Tab content extraction

/**
 * tabs.getText: Inject a content script to extract visible text from a tab.
 * @param {object} payload
 * @param {number} payload.tabId
 * @returns {Promise<{text: string}>}
 */
async function handleTabsGetText(payload) {
  if (!payload.tabId) throw new Error("tabs.getText: tabId is required");
  try {
    const results = await chrome.scripting.executeScript({
      target: { tabId: payload.tabId },
      func: () => {
        // Extract readable text from the page body
        const body = document.body;
        if (!body) return "";
        // Remove script/style elements from clone
        const clone = body.cloneNode(true);
        clone.querySelectorAll("script, style, noscript, svg, canvas").forEach(el => el.remove());
        return clone.innerText.substring(0, 8000); // limit to 8KB
      },
    });
    const text = results && results[0] && results[0].result ? results[0].result : "";
    return { text };
  } catch (e) {
    // Fallback: try fetching the page URL directly (works for some pages where executeScript fails)
    try {
      const tab = await chrome.tabs.get(payload.tabId);
      if (tab.url && (tab.url.startsWith("http://") || tab.url.startsWith("https://"))) {
        const resp = await fetch(tab.url, { credentials: "include" });
        const html = await resp.text();
        const text = html
          .replace(/<script[^>]*>[\s\S]*?<\/script>/gi, "")
          .replace(/<style[^>]*>[\s\S]*?<\/style>/gi, "")
          .replace(/<[^>]+>/g, " ")
          .replace(/\s+/g, " ")
          .trim()
          .substring(0, 8000);
        if (text) return { text };
      }
    } catch (_) {
      // fetch fallback also failed, return original error
    }
    return { text: "(cannot access page: " + e.message + ". Try reloading the extension at chrome://extensions)" };
  }
}

// Chrome API handlers — Tab controls
// ---------------------------------------------------------------------------

/**
 * tabs.mute: Toggle or set mute state for a tab.
 * @param {object} payload
 * @param {number} payload.tabId
 * @param {boolean} [payload.muted] - If omitted, toggles current state
 */
async function handleTabsMute(payload) {
  if (!payload.tabId) throw new Error("tabs.mute: tabId is required");
  let muted = payload.muted;
  if (muted === undefined) {
    const tab = await chrome.tabs.get(payload.tabId);
    muted = !tab.mutedInfo.muted;
  }
  const updated = await chrome.tabs.update(payload.tabId, { muted });
  return { tabId: updated.id, muted: updated.mutedInfo.muted };
}

/**
 * tabs.pin: Toggle or set pin state for a tab.
 * @param {object} payload
 * @param {number} payload.tabId
 * @param {boolean} [payload.pinned] - If omitted, toggles current state
 */
async function handleTabsPin(payload) {
  if (!payload.tabId) throw new Error("tabs.pin: tabId is required");
  let pinned = payload.pinned;
  if (pinned === undefined) {
    const tab = await chrome.tabs.get(payload.tabId);
    pinned = !tab.pinned;
  }
  const updated = await chrome.tabs.update(payload.tabId, { pinned });
  return { tabId: updated.id, pinned: updated.pinned };
}

/**
 * tabs.move: Move a tab to a different position or window.
 * @param {object} payload
 * @param {number} payload.tabId
 * @param {number} [payload.windowId] - Target window
 * @param {number} payload.index - Target position (-1 for end)
 */
async function handleTabsMove(payload) {
  if (!payload.tabId) throw new Error("tabs.move: tabId is required");
  const moveProps = { index: payload.index !== undefined ? payload.index : -1 };
  if (payload.windowId !== undefined) moveProps.windowId = payload.windowId;
  const moved = await chrome.tabs.move(payload.tabId, moveProps);
  return { tabId: moved.id, windowId: moved.windowId, index: moved.index };
}

/**
 * tabs.capture: Capture a screenshot of the active tab in a window.
 * @param {object} payload
 * @param {number} [payload.windowId] - Window to capture (current if omitted)
 * @param {string} [payload.format] - "png" or "jpeg" (default: "png")
 * @param {number} [payload.quality] - JPEG quality 0-100
 * @returns {Promise<{dataUrl: string}>}
 */
async function handleTabsCapture(payload) {
  const fmt = payload.format || "png";
  const options = { format: fmt };
  if (payload.quality !== undefined) options.quality = payload.quality;
  const windowId = payload.windowId || chrome.windows.WINDOW_ID_CURRENT;

  // Ensure the target tab is active and its window is focused (required for MV3 captureVisibleTab)
  if (payload.tabId) {
    try {
      await chrome.tabs.update(payload.tabId, { active: true });
      if (windowId !== chrome.windows.WINDOW_ID_CURRENT) {
        await chrome.windows.update(windowId, { focused: true });
      }
      // Brief delay for page to render after activation
      await new Promise(r => setTimeout(r, 150));
    } catch (_) {}
  }

  // Try captureVisibleTab first (works when extension has active host permissions)
  try {
    const dataUrl = await chrome.tabs.captureVisibleTab(windowId, options);
    return { dataUrl };
  } catch (e) {
    // Fallback: use chrome.debugger CDP Page.captureScreenshot
    if (payload.tabId) {
      try {
        await chrome.debugger.attach({ tabId: payload.tabId }, "1.3");
        const result = await chrome.debugger.sendCommand(
          { tabId: payload.tabId },
          "Page.captureScreenshot",
          { format: fmt === "jpeg" ? "jpeg" : "png", quality: payload.quality }
        );
        await chrome.debugger.detach({ tabId: payload.tabId });
        return { dataUrl: `data:image/${fmt};base64,${result.data}` };
      } catch (dbgErr) {
        try { await chrome.debugger.detach({ tabId: payload.tabId }); } catch (_) {}
        throw new Error(`captureVisibleTab: ${e.message}; debugger fallback: ${dbgErr.message}`);
      }
    }
    throw e;
  }
}

// ---------------------------------------------------------------------------
// Chrome API handlers — Bookmarks move
// ---------------------------------------------------------------------------

/**
 * bookmarks.move: Move a bookmark to a different folder/position.
 * @param {object} payload
 * @param {string} payload.id - Bookmark ID
 * @param {string} [payload.parentId] - New parent folder ID
 * @param {number} [payload.index] - Position within parent
 */
async function handleBookmarksMove(payload) {
  if (!payload.id) throw new Error("bookmarks.move: id is required");
  const dest = {};
  if (payload.parentId) dest.parentId = payload.parentId;
  if (payload.index !== undefined) dest.index = payload.index;
  const moved = await chrome.bookmarks.move(payload.id, dest);
  return {
    bookmark: {
      id: moved.id,
      title: moved.title || "",
      url: moved.url || "",
      parentId: moved.parentId || "",
    },
  };
}

// ---------------------------------------------------------------------------
// Chrome API handlers — Windows
// ---------------------------------------------------------------------------

/**
 * windows.list: Get all browser windows.
 */
async function handleWindowsList() {
  const wins = await chrome.windows.getAll({ populate: false });
  return {
    windows: wins.map((w) => ({
      id: w.id,
      focused: w.focused,
      state: w.state,
      type: w.type,
      width: w.width,
      height: w.height,
      top: w.top,
      left: w.left,
    })),
  };
}

/**
 * windows.create: Create a new browser window.
 * @param {object} payload
 * @param {string} [payload.url] - URL to open
 * @param {boolean} [payload.focused]
 * @param {string} [payload.state] - "normal", "minimized", "maximized", "fullscreen"
 */
async function handleWindowsCreate(payload) {
  const props = {};
  if (payload.url) props.url = payload.url;
  if (payload.focused !== undefined) props.focused = payload.focused;
  if (payload.state) props.state = payload.state;
  const win = await chrome.windows.create(props);
  return { windowId: win.id, state: win.state };
}

/**
 * windows.close: Close a browser window.
 * @param {object} payload
 * @param {number} payload.windowId
 */
async function handleWindowsClose(payload) {
  if (!payload.windowId) throw new Error("windows.close: windowId is required");
  await chrome.windows.remove(payload.windowId);
  return { closed: true };
}

/**
 * windows.focus: Focus the last-focused Chrome window (bring browser to front).
 */
async function handleWindowsFocus() {
  const win = await chrome.windows.getLastFocused();
  if (win && win.id) {
    await chrome.windows.update(win.id, { focused: true });
  }
  return { focused: true };
}

// ---------------------------------------------------------------------------
// Chrome API handlers — History
// ---------------------------------------------------------------------------

/**
 * history.search: Search browsing history.
 * @param {object} payload
 * @param {string} payload.query - Search text
 * @param {number} [payload.maxResults] - Max results (default 100)
 * @param {number} [payload.startTime] - Start time in ms since epoch
 * @param {number} [payload.endTime] - End time in ms since epoch
 */
async function handleHistorySearch(payload) {
  if (!payload.query && payload.query !== "") throw new Error("history.search: query is required");
  const searchParams = {
    text: payload.query,
    maxResults: payload.maxResults || 100,
  };
  if (payload.startTime) searchParams.startTime = payload.startTime;
  if (payload.endTime) searchParams.endTime = payload.endTime;
  const results = await chrome.history.search(searchParams);
  return {
    history: results.map((h) => ({
      id: h.id,
      url: h.url || "",
      title: h.title || "",
      lastVisitTime: h.lastVisitTime,
      visitCount: h.visitCount,
      typedCount: h.typedCount,
    })),
  };
}

/**
 * history.delete: Delete a URL from history.
 * @param {object} payload
 * @param {string} payload.url - URL to delete
 */
async function handleHistoryDelete(payload) {
  if (!payload.url) throw new Error("history.delete: url is required");
  await chrome.history.deleteUrl({ url: payload.url });
  return { deleted: true };
}

// ---------------------------------------------------------------------------
// Chrome API handlers — Downloads
// ---------------------------------------------------------------------------

/**
 * downloads.list: List recent downloads.
 * @param {object} payload
 * @param {string} [payload.query] - Search filename
 * @param {number} [payload.limit] - Max results (default 50)
 */
async function handleDownloadsList(payload) {
  const searchParams = { limit: (payload && payload.limit) || 50, orderBy: ["-startTime"] };
  if (payload && payload.query) searchParams.filenameRegex = payload.query;
  const items = await chrome.downloads.search(searchParams);
  return {
    downloads: items.map((d) => ({
      id: d.id,
      filename: d.filename || "",
      url: d.url || "",
      state: d.state,
      totalBytes: d.totalBytes,
      bytesReceived: d.bytesReceived,
      startTime: d.startTime,
      endTime: d.endTime || "",
      mime: d.mime || "",
      danger: d.danger || "safe",
    })),
  };
}

/**
 * downloads.cancel: Cancel an in-progress download.
 * @param {object} payload
 * @param {number} payload.id - Download ID
 */
async function handleDownloadsCancel(payload) {
  if (!payload.id) throw new Error("downloads.cancel: id is required");
  await chrome.downloads.cancel(payload.id);
  return { cancelled: true };
}

/**
 * bookmarks.remove: Remove a bookmark or folder by ID.
 * @param {object} payload
 * @param {string} payload.id - Bookmark/folder ID
 * @returns {Promise<{removed: boolean}>}
 */
async function handleBookmarksRemove(payload) {
  if (!payload.id) {
    throw new Error("bookmarks.remove: id is required");
  }
  // Use removeTree for folders (handles non-empty folders too)
  try {
    await chrome.bookmarks.removeTree(payload.id);
  } catch (e) {
    // If removeTree fails (e.g. trying to remove root), try regular remove
    await chrome.bookmarks.remove(payload.id);
  }
  return { removed: true };
}

/**
 * bookmarks.create: Create a new bookmark or folder.
 * @param {object} payload
 * @param {string} payload.parentId - Parent folder ID
 * @param {string} [payload.title] - Bookmark title
 * @param {string} [payload.url] - URL (omit for folder)
 * @param {number} [payload.index] - Position within parent
 * @returns {Promise<{bookmark: object}>}
 */
async function handleBookmarksCreate(payload) {
  if (!payload.parentId) {
    throw new Error("bookmarks.create: parentId is required");
  }
  const createProps = { parentId: payload.parentId };
  if (payload.title) createProps.title = payload.title;
  if (payload.url) createProps.url = payload.url;
  if (payload.index !== undefined) createProps.index = payload.index;
  const b = await chrome.bookmarks.create(createProps);
  return {
    bookmark: {
      id: b.id,
      title: b.title || "",
      url: b.url || "",
      parentId: b.parentId || "",
      dateAdded: b.dateAdded,
    },
  };
}

/**
 * bookmarks.update: Update a bookmark's title or URL.
 * @param {object} payload
 * @param {string} payload.id - Bookmark ID
 * @param {string} [payload.title] - New title
 * @param {string} [payload.url] - New URL
 * @returns {Promise<{bookmark: object}>}
 */
async function handleBookmarksUpdate(payload) {
  if (!payload.id) {
    throw new Error("bookmarks.update: id is required");
  }
  const changes = {};
  if (payload.title !== undefined) changes.title = payload.title;
  if (payload.url !== undefined) changes.url = payload.url;
  const b = await chrome.bookmarks.update(payload.id, changes);
  return {
    bookmark: {
      id: b.id,
      title: b.title || "",
      url: b.url || "",
      parentId: b.parentId || "",
      dateAdded: b.dateAdded,
    },
  };
}

// ---------------------------------------------------------------------------
// Message sending helpers
// ---------------------------------------------------------------------------

/**
 * Send a raw message object over the native port.
 * No-op if the port is not connected.
 * @param {object} msg - Message to send
 */
function sendRaw(msg) {
  if (!port) {
    warn("Cannot send: port is null");
    return;
  }
  try {
    port.postMessage(msg);
  } catch (err) {
    warn("postMessage failed:", err.message);
  }
}

/**
 * Send a success response back to the daemon.
 * @param {string} id - Request ID to correlate with
 * @param {string} action - Action name
 * @param {object} payload - Response payload
 */
function sendResponse(id, action, payload) {
  sendRaw({
    id,
    protocol_version: PROTOCOL_VERSION,
    type: "response",
    action,
    payload,
  });
}

/**
 * Send an error response back to the daemon.
 * @param {string} id - Request ID to correlate with
 * @param {string} action - Action name
 * @param {string} code - Error code
 * @param {string} message - Human-readable error message
 */
function sendError(id, action, code, message) {
  sendRaw({
    id,
    protocol_version: PROTOCOL_VERSION,
    type: "error",
    action,
    error: { code, message },
  });
}

/**
 * Send an event message to the daemon.
 * Events include `_target.targetId` so the daemon knows which browser
 * instance generated the event.
 * @param {string} action - Event action (e.g. "tabs.created")
 * @param {object} payload - Event payload (will have _target injected)
 */
function sendEvent(action, payload) {
  // Inject _target into every event payload
  payload._target = { targetId: targetId || "" };

  sendRaw({
    id: makeId(),
    protocol_version: PROTOCOL_VERSION,
    type: "event",
    action,
    payload,
  });
}

// ---------------------------------------------------------------------------
// Chrome event listeners — Tabs
// ---------------------------------------------------------------------------

chrome.tabs.onCreated.addListener((tab) => {
  sendEvent("tabs.created", {
    tab: {
      id: tab.id,
      windowId: tab.windowId,
      index: tab.index,
      title: tab.title || "",
      url: tab.url || "",
      active: tab.active,
      pinned: tab.pinned,
      groupId: tab.groupId,
      favIconUrl: tab.favIconUrl || "",
    },
  });
});

chrome.tabs.onRemoved.addListener((tabId, removeInfo) => {
  sendEvent("tabs.removed", {
    tabId,
    removeInfo,
  });
});

chrome.tabs.onUpdated.addListener((tabId, changeInfo, tab) => {
  sendEvent("tabs.updated", {
    tabId,
    changeInfo,
    tab: {
      id: tab.id,
      windowId: tab.windowId,
      index: tab.index,
      title: tab.title || "",
      url: tab.url || "",
      active: tab.active,
      pinned: tab.pinned,
      groupId: tab.groupId,
      favIconUrl: tab.favIconUrl || "",
    },
  });
});

chrome.tabs.onActivated.addListener((activeInfo) => {
  sendEvent("tabs.activated", {
    tabId: activeInfo.tabId,
    windowId: activeInfo.windowId,
  });
});

chrome.tabs.onMoved.addListener((tabId, moveInfo) => {
  sendEvent("tabs.moved", {
    tabId,
    moveInfo,
  });
});

// ---------------------------------------------------------------------------
// Chrome event listeners — Bookmarks
// ---------------------------------------------------------------------------

chrome.bookmarks.onCreated.addListener((id, bookmark) => {
  sendEvent("bookmarks.created", {
    id,
    bookmark: {
      id: bookmark.id,
      title: bookmark.title || "",
      url: bookmark.url || "",
      parentId: bookmark.parentId || "",
      dateAdded: bookmark.dateAdded,
    },
  });
});

chrome.bookmarks.onChanged.addListener((id, changeInfo) => {
  sendEvent("bookmarks.changed", {
    id,
    changes: changeInfo,
  });
});

chrome.bookmarks.onRemoved.addListener((id, removeInfo) => {
  sendEvent("bookmarks.removed", {
    id,
    removeInfo,
  });
});

// ---------------------------------------------------------------------------
// Outgoing requests to daemon
// ---------------------------------------------------------------------------

/**
 * Send a request to the daemon and return a Promise for the response.
 * @param {string} action - Action name (e.g. "collections.list")
 * @param {object} [payload] - Request payload
 * @returns {Promise<object>} - Response payload
 */
function sendDaemonRequest(action, payload) {
  return new Promise((resolve, reject) => {
    const id = makeId();
    pendingRequests.set(id, { resolve, reject });
    sendRaw({
      id,
      protocol_version: PROTOCOL_VERSION,
      type: "request",
      action,
      payload: payload || {},
    });
    // Timeout after 10 seconds
    setTimeout(() => {
      if (pendingRequests.has(id)) {
        pendingRequests.delete(id);
        reject(new Error(`Request ${action} timed out`));
      }
    }, 10000);
  });
}

// ---------------------------------------------------------------------------
// Context Menu — Add to Collection
// ---------------------------------------------------------------------------

const MENU_PARENT_ID = "ctm-add-to-collection";

/**
 * Fetch collections from daemon and rebuild the context menu.
 */
async function refreshCollectionsMenu() {
  try {
    await chrome.contextMenus.removeAll();
  } catch (_) {}

  try {
    const result = await sendDaemonRequest("collections.list", {});
    const collections = result.collections || [];

    chrome.contextMenus.create({
      id: MENU_PARENT_ID,
      title: "Add to CTM Collection",
      contexts: ["page", "link"],
    });

    if (collections.length === 0) {
      chrome.contextMenus.create({
        id: "ctm-no-collections",
        parentId: MENU_PARENT_ID,
        title: "(no collections)",
        enabled: false,
        contexts: ["page", "link"],
      });
    } else {
      for (const col of collections) {
        chrome.contextMenus.create({
          id: `ctm-col:${col.name}`,
          parentId: MENU_PARENT_ID,
          title: `${col.name} (${col.itemCount})`,
          contexts: ["page", "link"],
        });
      }
    }

    log(`Collections menu built: ${collections.length} item(s)`);
  } catch (err) {
    warn("Failed to build collections menu:", err.message);
  }
}

chrome.contextMenus.onClicked.addListener(async (info, tab) => {
  const menuId = String(info.menuItemId);
  if (!menuId.startsWith("ctm-col:")) return;

  const collectionName = menuId.slice("ctm-col:".length);

  // Determine URL and title from context
  let url, title;
  if (info.linkUrl) {
    url = info.linkUrl;
    title = info.selectionText || info.linkUrl;
  } else {
    url = tab.url;
    title = tab.title || tab.url;
  }

  try {
    await sendDaemonRequest("collections.addItems", {
      name: collectionName,
      items: [{ url, title }],
    });
    log(`Added to "${collectionName}": ${title}`);
    // Refresh menu to update item counts
    refreshCollectionsMenu();
  } catch (err) {
    warn(`Failed to add to "${collectionName}":`, err.message);
  }
});

// ---------------------------------------------------------------------------
// Keyboard Shortcut — Add to Collection Picker
// ---------------------------------------------------------------------------

/**
 * Inject a lightweight collection picker overlay into the active tab.
 * The user clicks a collection name; the content script sends a message
 * back to the service worker to complete the add.
 *
 * @param {string[]} collections - Collection names to show
 */
function showCollectionPicker(collections) {
  // Remove existing picker if any
  const existing = document.getElementById("ctm-collection-picker");
  if (existing) existing.remove();

  const overlay = document.createElement("div");
  overlay.id = "ctm-collection-picker";
  overlay.style.cssText =
    "position:fixed;top:0;left:0;right:0;bottom:0;" +
    "background:rgba(0,0,0,0.45);z-index:2147483647;" +
    "display:flex;align-items:flex-start;justify-content:center;" +
    "padding-top:80px;font-family:-apple-system,BlinkMacSystemFont,sans-serif;";

  const box = document.createElement("div");
  box.style.cssText =
    "background:#fff;border-radius:10px;padding:16px 12px;" +
    "min-width:280px;max-height:420px;overflow-y:auto;" +
    "box-shadow:0 8px 32px rgba(0,0,0,0.28);";

  const heading = document.createElement("div");
  heading.textContent = "Add to Collection";
  heading.style.cssText =
    "font-size:15px;font-weight:600;color:#333;margin-bottom:10px;padding:0 4px;";
  box.appendChild(heading);

  let focusIdx = 0;
  const items = [];

  collections.forEach((name, i) => {
    const row = document.createElement("div");
    row.textContent = name;
    row.style.cssText =
      "padding:9px 10px;cursor:pointer;border-radius:6px;" +
      "margin:2px 0;font-size:14px;color:#222;";
    row.addEventListener("mouseenter", () => {
      focusIdx = i;
      highlightItem();
    });
    row.addEventListener("click", () => {
      chrome.runtime.sendMessage({ type: "ctm-pick-collection", name });
      overlay.remove();
    });
    items.push(row);
    box.appendChild(row);
  });

  function highlightItem() {
    items.forEach((el, i) => {
      el.style.background = i === focusIdx ? "#e8f0fe" : "transparent";
    });
  }
  highlightItem();

  function onKey(e) {
    if (e.key === "Escape") {
      overlay.remove();
      document.removeEventListener("keydown", onKey, true);
      e.stopPropagation();
    } else if (e.key === "ArrowDown" || e.key === "j") {
      focusIdx = (focusIdx + 1) % items.length;
      highlightItem();
      e.preventDefault();
    } else if (e.key === "ArrowUp" || e.key === "k") {
      focusIdx = (focusIdx - 1 + items.length) % items.length;
      highlightItem();
      e.preventDefault();
    } else if (e.key === "Enter") {
      const name = collections[focusIdx];
      chrome.runtime.sendMessage({ type: "ctm-pick-collection", name });
      overlay.remove();
      document.removeEventListener("keydown", onKey, true);
      e.preventDefault();
    }
  }
  document.addEventListener("keydown", onKey, true);

  overlay.addEventListener("click", (e) => {
    if (e.target === overlay) {
      overlay.remove();
      document.removeEventListener("keydown", onKey, true);
    }
  });

  overlay.appendChild(box);
  document.body.appendChild(overlay);
}

chrome.commands.onCommand.addListener(async (command) => {
  if (command !== "add-to-collection") return;

  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (!tab) return;

  try {
    const result = await sendDaemonRequest("collections.list", {});
    const names = (result.collections || []).map((c) => c.name);

    if (names.length === 0) {
      warn("Shortcut: no collections available");
      return;
    }

    await chrome.scripting.executeScript({
      target: { tabId: tab.id },
      func: showCollectionPicker,
      args: [names],
    });
  } catch (err) {
    warn("Shortcut: failed to show picker:", err.message);
  }
});

// Handle picker selection from content script
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (message.type !== "ctm-pick-collection") return;

  const tab = sender.tab;
  if (!tab) return;

  sendDaemonRequest("collections.addItems", {
    name: message.name,
    items: [{ url: tab.url, title: tab.title || tab.url }],
  })
    .then(() => {
      log(`Picker: added to "${message.name}": ${tab.title}`);
      refreshCollectionsMenu();
    })
    .catch((err) => {
      warn(`Picker: failed to add to "${message.name}":`, err.message);
    });
});

// ---------------------------------------------------------------------------
// Lifecycle — Service worker startup and keepalive
// ---------------------------------------------------------------------------

// Service workers can be terminated at any time. These events ensure we
// reconnect whenever Chrome restarts or the extension is (re)installed.

chrome.runtime.onStartup.addListener(() => {
  log("onStartup fired");
  connect();
});

chrome.runtime.onInstalled.addListener((details) => {
  log("onInstalled fired, reason:", details.reason);
  connect();
});

// Initial connection when the service worker script first loads.
// This covers the case where the service worker is loaded for the first time
// (not via onStartup or onInstalled, e.g., after being terminated and woken
// by an event).
connect();

log("Service worker loaded");
