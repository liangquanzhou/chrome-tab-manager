# CTM — Bookmarks Model

领域定义见 `02_DOMAIN.md`（BookmarkSource / BookmarkMirror / BookmarkOverlay）。
产品原则见 `01_PRODUCT.md` P3。

## 1. Three Bookmark Layers

| Layer | Owner | Role | Sync |
|-------|-------|------|------|
| **BookmarkSource** | Chrome / Google | 提供原始数据 | Google Sync |
| **BookmarkMirror** | CTM local | 统一搜索/浏览/workspace/export | optional via iCloud |
| **BookmarkOverlay** | CTM local | tags / notes / aliases / workspace relationships | iCloud |

三层分开，Chrome/Google Sync 与 CTM library sync 互不污染。

## 2. Dual Source of Truth

- **Native truth**: 原生书签结构，由 Chrome / Google Sync 维护
- **CTM truth**: 对书签的增强解释和组织，由 CTM library 维护

两者不能混成一层。

## 3. CTM Bookmark Capabilities

CTM 不取代 Chrome 书签系统，补齐 Chrome 没做好的：

- Terminal tree browsing
- Unified search (跨 tabs / sessions / collections / bookmarks)
- Tag / note / alias overlay
- Workspace attachment
- Cross-resource linking
- Export (HTML / JSON / Markdown)

## 4. Bookmarks in Search

书签必须天然进入统一搜索。搜索结果同时支持原生属性和 CTM overlay 属性。

## 5. Bookmarks in Workspace

书签能：
- 进入 workspace
- 与 session / collection 并列存在
- 承载 task-specific tags / notes

## 6. Bookmarks in Sync

| 层 | 同步方 |
|----|--------|
| 原生书签 | Google / Chrome |
| BookmarkOverlay | CTM + iCloud |

原生 bookmark 树不由 CTM 云端重新定义，但 CTM 对书签的增强能力跨设备同步。

## 7. Bookmark User Journeys

| Journey | Flow |
|---------|------|
| A | Bookmark → search → open in tab |
| B | Bookmark → attach to workspace |
| C | Bookmark → add tag/note → long-term knowledge asset |
| D | Bookmark set → export / collection / startup material |

## 8. Red Flags

- bookmarks 只是只读列表
- bookmarks 不参与统一搜索
- bookmarks 不能附加 note/tag
- bookmarks 不能进入 workspace
- CTM 想接管原生书签同步本身
