# CTM — Search Model

## 1. Purpose

这份文档定义 CTM 的**搜索模型**。

它回答的是：

- 搜索到底是不是产品的一等能力
- 搜索覆盖哪些资源域
- 搜索结果如何组织
- 搜索与 saved search / smart collection / workspace 的关系是什么

---

## 2. Search Philosophy

CTM 的搜索不应该只是：

- 某个 view 的文本过滤
- 一个附属输入框

而应该是：

**跨资源统一入口**

用户不需要先知道某个东西属于：

- tab
- session
- collection
- bookmark
- workspace

他只需要知道：

- “我想找它”

---

## 3. Search Domains

CTM 的统一搜索至少应覆盖这 5 个域：

1. `Tabs`
2. `Sessions`
3. `Collections`
4. `Bookmarks`
5. `Workspaces`

以后还可以扩展到：

- tags
- notes
- saved searches

---

## 4. Searchable Dimensions

搜索不应只按 title。

至少应支持这些维度：

- name
- title
- url
- host / domain
- tag
- note
- alias
- resource type
- workspace membership

---

## 5. Search Modes

CTM 的搜索应至少有三种模式。

## 5.1 Quick Search

用于快速定位某个资源。

### Characteristics

- 输入即搜索
- 面向当前意图
- 更偏即时动作

### Typical use

- 我想快速找某个 tab / bookmark

---

## 5.2 Global Search

用于跨资源统一检索。

### Characteristics

- 一次搜索多个资源域
- 结果按资源类型分组或混合排序
- 是产品级入口

### Typical use

- 我记得这个链接可能在 session、collection 或 bookmark 里

---

## 5.3 Saved Search

用于保存可重复使用的查询定义。

### Characteristics

- 可命名
- 可复用
- 可转化为 smart collection 或 workspace workflow

### Typical use

- 找所有带某个 tag 的资源
- 找某个 domain 的全部资料

---

## 6. Search Result Types

搜索结果不应只有一种扁平 item。

从产品角度，结果至少要区分：

- Tab result
- Session result
- Collection result
- Bookmark result
- Workspace result

这样用户才能明确知道：

- 这个结果是什么
- 打开它会发生什么

---

## 7. Search Result Actions

搜索不只是找到结果，还应支持下一步行动。

### Actions by result type

- Tab → activate / close / add to collection
- Session → preview / restore / attach to workspace
- Collection → expand / restore / export
- Bookmark → open / tag / attach / add note
- Workspace → enter / startup / search within

这意味着：

搜索是行动入口，而不是只读列表。

---

## 8. Search + Workspace Relationship

workspace 不应和 search 脱节。

搜索至少有两种 workspace 关系：

1. **Search across workspaces**
   - 某个资源属于哪个 workspace

2. **Search within a workspace**
   - 只在某个 workspace 作用域内搜索

这样 workspace 才不会只是“一个静态容器”。

---

## 9. Search + Bookmarks Relationship

书签必须天然参与统一搜索。

### Why this matters

- bookmarks 是长期知识
- bookmarks 常常是搜索目标，而不是手工翻树
- bookmarks 可以带 overlay 数据

因此：

- search 不应把 bookmark 看成二等公民
- bookmark result 应同时支持原生属性和 CTM overlay 属性

---

## 10. Search + Library Relationship

library 对象天然是搜索核心。

### Sessions

搜索它们的意义是：

- 找回过去的工作现场

### Collections

搜索它们的意义是：

- 找回人工整理的链接包

### Workspaces

搜索它们的意义是：

- 找到完整任务上下文

---

## 11. Smart Collections

当搜索足够稳定之后，应自然支持：

`Smart Collection`

### Definition

不是手工维护 items 的 collection，而是由查询定义驱动的 collection。

### Example

- 所有带 `research` tag 的 bookmarks
- 所有 `openai.com` 域名的资源
- 某个 workspace 下最近修改的资源

这会让 search 从“找东西”升级成“定义资源视图”。

---

## 12. Saved Search

Saved Search 是 CTM 搜索模型中的长期对象。

### Value

- 让高频查询可复用
- 让查询可以参与 workspace
- 让搜索成为长期组织能力的一部分

### Relationship

- saved search 可以独立存在
- 也可以成为 smart collection 或 workspace 的一部分

---

## 13. Ranking and Presentation

搜索结果的高层呈现应考虑三件事：

1. **结果属于什么资源域**
2. **结果与当前上下文的相关度**
3. **用户能立即执行什么动作**

也就是说，搜索展示不应只是“文本匹配列表”。

---

## 14. Search Red Flags

如果未来出现这些情况，说明搜索模型退化了：

- 搜索只剩 Tabs 页里的过滤框
- bookmark 不能进入统一搜索
- workspace 不能进入统一搜索
- 搜索结果不能直接行动
- 没有 saved search
- 没有 smart collection

---

## 15. Final Search Statement

CTM 的搜索应始终被定义为：

**跨 tabs、sessions、collections、bookmarks、workspaces 的统一行动入口。**

它不是附属功能，而是完整产品的中心能力之一。
