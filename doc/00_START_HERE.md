# CTM — Start Here

## What Is CTM

**CTM = terminal-first browser workspace manager**

单二进制 Go 程序，通过 Chrome Extension + Native Messaging + Unix Socket 实现终端对浏览器的完整控制、长期资源管理、跨设备同步。

## doc/ 文件概览

### 产品设计（做什么）

| # | 文件 | 内容 |
|---|------|------|
| 01 | `01_PRODUCT.md` | 产品定义、四层模型、Source of Truth、原则、决策日志 |
| 02 | `02_DOMAIN.md` | 16 个领域对象、属性、生命周期、关系图 |
| 03 | `03_CAPABILITIES.md` | 7 大能力域、8 个 build 阶段、完整 feature 列表 |
| 04 | `04_INTERACTION.md` | 9 个导航区域、CLI/TUI/命令面板、10 条用户旅程 |

### 子系统设计（复杂领域深潜）

| # | 文件 | 内容 |
|---|------|------|
| 05 | `05_BOOKMARKS.md` | 三层模型：Source → Mirror → Overlay |
| 06 | `06_SEARCH.md` | 3 种搜索模式、9 个维度、Smart Collection |
| 07 | `07_SYNC.md` | 双轨同步、7 种状态、冲突处理 |
| 08 | `08_WORKSPACE.md` | Workspace 定义、生命周期、模板 |

### 工程实现（怎么写代码）

| # | 文件 | 内容 |
|---|------|------|
| 09 | `09_DESIGN.md` | Go 模块设计、依赖图、struct 定义、状态机 |
| 10 | `10_TUI.md` | 按键定义、输入模式状态机、三通道反馈、布局 |
| 11 | `11_PLAN.md` | 架构图、技术选型、8 个 Phase、验证标准 |
| 12 | `12_CONTRACTS.md` | 每个 daemon action 的精确 JSON request/response |

### 质量与流程（怎么验收、怎么协作）

| # | 文件 | 内容 |
|---|------|------|
| 13 | `13_ACCEPTANCE.md` | 每个 Phase 的自动化测试 + 手动验证清单 |
| 14 | `14_LESSONS.md` | TS 版 12 条教训 + Go 重写 checklist |
| 15 | `15_AGENT_RULES.md` | Agent 操作守则：防止偷偷缩小产品范围 |
| 16 | `16_CHANGE_POLICY.md` | 需求变更流程：先改文档再改代码 |
| 17 | `17_TEMPLATES.md` | 派活模板 + 验收模板 |
| 18 | `18_CURRENT_PHASE.md` | 当前 Phase 状态：做什么、不做什么、exit criteria |

---

## 阅读顺序

### 人类 Owner（理解产品）

1. `01_PRODUCT.md` — 产品是什么
2. `02_DOMAIN.md` — 有哪些实体
3. `04_INTERACTION.md` — 用户怎么用
4. `11_PLAN.md` — 怎么分阶段建
5. `15_AGENT_RULES.md` — 怎么管 agent

### Agent（开始写代码）

1. `01_PRODUCT.md` — 理解产品边界
2. `02_DOMAIN.md` — 理解领域对象
3. `03_CAPABILITIES.md` — 理解能力域和 build 顺序
4. `09_DESIGN.md` — 理解模块职责和依赖
5. `12_CONTRACTS.md` — 查 API 精确格式
6. `10_TUI.md` — 查 TUI 按键和交互
7. `13_ACCEPTANCE.md` — 确认验收标准
8. `14_LESSONS.md` — 避坑
9. `15_AGENT_RULES.md` — 操作守则

### 查阅特定子系统

书签 → `05_BOOKMARKS.md`，搜索 → `06_SEARCH.md`，同步 → `07_SYNC.md`，工作空间 → `08_WORKSPACE.md`

---

## 核心原则

- **doc/ 是唯一规格来源**。写代码前先读文档，不可凭记忆猜测。
- **每个信息点只在一处定义**。其他文件通过交叉引用（如 "见 `02_DOMAIN.md`"）链接。
- **先改文档，再改代码**。需求变更流程见 `16_CHANGE_POLICY.md`。
