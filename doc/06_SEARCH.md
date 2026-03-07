# CTM — Search Model

产品原则见 `01_PRODUCT.md` P4。

## 1. Search = Cross-Resource Unified Entry

用户不需要先知道资源属于 tab / session / collection / bookmark / workspace，只需要"我想找它"。

## 2. Search Domains

1. Tabs
2. Sessions
3. Collections
4. Bookmarks
5. Workspaces

未来可扩展到 tags / notes / saved searches。

## 3. Searchable Dimensions

- name / title / url / host (domain)
- tag / note / alias
- resource type / workspace membership

## 4. Three Search Modes

| Mode | 特征 | 典型用途 |
|------|------|---------|
| **Quick Search** | 输入即搜索，偏即时动作 | 快速找某个 tab / bookmark |
| **Global Search** | 一次搜索多个资源域，产品级入口 | "这个链接可能在 session、collection 或 bookmark 里" |
| **Saved Search** | 可命名、可复用、可转 smart collection 或 workspace workflow | 所有带某 tag 的资源、某域名全部资料 |

## 5. Search Result Types

结果区分 resource kind：Tab / Session / Collection / Bookmark / Workspace。用户能明确知道结果是什么、打开后会发生什么。

## 6. Search Result Actions

| Result Kind | 可用 Actions |
|-------------|-------------|
| Tab | activate / close / add to collection |
| Session | preview / restore / attach to workspace |
| Collection | expand / restore / export |
| Bookmark | open / tag / attach / add note |
| Workspace | enter / startup / search within |

搜索是行动入口，不是只读列表。

## 7. Search + Workspace

两种关系：
1. **Search across workspaces** — 某个资源属于哪个 workspace
2. **Search within a workspace** — 在某 workspace 作用域内搜索

## 8. Smart Collections

查询定义驱动的动态 collection（不手工维护 items）。

示例：
- 所有带 `research` tag 的 bookmarks
- 所有 `openai.com` 域名的资源
- 某 workspace 下最近修改的资源

## 9. Ranking & Presentation

搜索结果考虑三件事：
1. 结果属于什么资源域
2. 结果与当前上下文的相关度
3. 用户能立即执行什么动作

## 10. Red Flags

- 搜索只剩 Tabs 页的过滤框
- bookmark 不能进入统一搜索
- workspace 不能进入统一搜索
- 搜索结果不能直接行动
- 没有 saved search / smart collection
