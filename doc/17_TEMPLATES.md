# CTM — Task & Review Templates

本文件提供两个标准模板，供 owner 派活和验收使用。

---

## Part A: 派活模板

### Owner 层（简单版）

Owner 只需填这段：

```md
Task:

What I want:
-

Why I want it:
-

What should feel different after this is done:
-

Anything I definitely want included:
-

Anything I definitely do not want:
-
```

### Agent 展开层（编码前必须完成）

收到 owner 请求后，agent 必须展开为以下结构，**展开完成前不开始编码**：

```md
# Task Title

## 1. Requested Outcome
- 完成后必须存在或改变的行为

## 2. Product Area
- Runtime / Library / Bookmarks / Search / Workspace / Sync / Power
- 如跨多个域，列出 Primary + Secondary

## 3. Domain Objects Affected
- 涉及哪些对象（见 02_DOMAIN.md）

## 4. User Journey Affected
- 改变了哪些用户旅程（见 04_INTERACTION.md）

## 5. Command Surfaces Affected
- CLI / TUI / Command Palette

## 6. Source-of-Truth Impact
- 触及哪个 truth 边界（见 01_PRODUCT.md §3）

## 7. In Scope
- 本次任务包含的精确项

## 8. Out of Scope
- 本次任务刻意排除的项

## 9. Docs To Read First
- 列出需要先读的 doc/ 文件

## 10. Docs That Must Be Updated
- 如实现改变了含义，哪些文档必须更新

## 11. Acceptance Criteria
- 什么条件下算完成

## 12. Risks / Change Sensitivity
- 什么可能意外缩窄产品或破坏未来扩展性
```

### 完成汇报模板

实现完成后，agent 应以此格式汇报：

```md
## Completed
-

## Product areas touched
-

## Domain objects touched
-

## Docs updated
-

## Acceptance evidence
-

## Remaining risks
-
```

### 坏味道识别

如果任务 prompt 包含以下措辞，agent 应放慢展开：

- "就加一个..."
- "就让它支持..."
- "就同步一下..."
- "就加个视图..."

这些通常隐藏了模型级变更。

---

## Part B: 验收模板

### 验收输出格式

```md
## Verdict
- accepted / conditionally accepted / not accepted

## What changed
-

## Product areas touched
-

## Domain objects touched
-

## Findings
- 按严重性排序

## Acceptance evidence
- 运行了什么命令
- 观察到什么行为
- 更新了什么文档

## Open questions
-

## Residual risks
-

## Next actions
-
```

### 验收维度

每次严肃验收必须检查以下所有维度：

| 维度 | 检查问题 |
|------|----------|
| **Product Fit** | 是否增强或削弱了产品定义？有没有一等域被偷偷降级？ |
| **Domain Fit** | 对象正确吗？Source of Truth 保持了吗？Overlay 和原生数据分离了吗？ |
| **UX Fit** | 新行为可发现吗？符合导航和 command surface 设计吗？ |
| **Sync/Search/Workspace Fit** | 该可搜索的可搜索了吗？该 sync-aware 的 sync-aware 了吗？ |
| **Change Safety** | 让未来变更更容易还是更难？引入了隐藏耦合吗？ |

### 严重性定义

| 级别 | 含义 |
|------|------|
| **High** | 阻断验收：破坏产品方向、核心行为、Source of Truth、未来可扩展性 |
| **Medium** | 不完全阻断，但在 UX 一致性、领域一致性、信心方面留有风险 |
| **Low** | 清理、打磨、文档补充，不改变主要结果 |

### Owner 可以问的验收问题

如果你不读代码，可以直接问 agent 这些问题：

1. 产品哪个部分变好了？
2. 哪些核心对象改变了？
3. 有没有触及 Bookmarks / Search / Workspace / Sync？
4. 文档是在代码之前还是之后更新的？
5. 因为这次工作，什么东西未来更难改了？
6. 这是真的完成了，还是只是部分完成？
7. 接下来应该测试什么？

### 验收结论

| 结论 | 含义 |
|------|------|
| **Accepted** | 行为正确、符合产品模型、可以安全地在此基础上继续构建 |
| **Conditionally Accepted** | 主要行为正确，但有一小组后续问题需要修复 |
| **Not Accepted** | 实现可能存在，但尚不够安全或对齐，不能作为稳定基础 |
