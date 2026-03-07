# CTM — Agent Execution Rules

本文件面向 AI agent，不面向终端用户。Owner 是非技术用户，主要提供需求和产品方向。

## 1. 首要原则

**不要从代码推断产品方向。始终从 doc/ 文档推导。**

代码是实现快照，文档定义产品应该成为什么。

## 2. 编码前必读

开始任何有意义的实现前，agent 必须按顺序阅读：

1. `01_PRODUCT.md` — 产品定义和原则
2. `02_DOMAIN.md` — 领域对象和生命周期
3. `03_CAPABILITIES.md` — 能力域和 build 顺序
4. `09_DESIGN.md` — 模块职责和依赖图
5. `12_CONTRACTS.md` — API 精确格式
6. `13_ACCEPTANCE.md` — 当前 Phase 验收标准

涉及 TUI 时加读 `10_TUI.md`。涉及子系统时加读 `05`-`08`。

## 3. 产品意图守则

Agent 必须始终保持以下假设：

1. CTM 是完整产品，不是小工具
2. Bookmarks、Search、Workspace、Sync、Power 都是产品范围内的能力域
3. Owner 是非技术用户，主要提供需求和产品方向
4. 文档存在的目的是降低未来变更成本
5. 不要为了短期编码效率而缩窄未来的产品模型

## 4. 绝对禁止

Agent 绝对不能：

- 偷偷把 Bookmarks 降级为边缘功能
- 偷偷把 Search 实现为局部过滤而非跨资源搜索
- 偷偷把 Workspace 坍缩为 Collection/Session 的别名
- 偷偷把 Sync 隐藏为纯后台机制
- 偷偷根据当前部分代码重新定义产品范围
- 偷偷假设 owner 接受一个更窄的产品

如果实现困难，agent 可以分阶段实施。但不可以偷偷缩小产品。

## 5. 必须显式保护

Agent 必须显式保护：

- Source of Truth 边界（见 `01_PRODUCT.md` §3）
- 对象身份（UUID + updatedAt）
- Workspace 中心地位
- Local-first 行为
- Sync 可见性
- 跨资源可搜索性
- CLI/TUI/Palette 语义一致性

## 6. 重大变更前的必答问题

实现重要功能或重构前，agent 必须回答：

1. 属于哪个产品域？（Runtime / Library / Bookmarks / Search / Workspace / Sync / Power）
2. 影响哪些领域对象？
3. 受影响数据的 Source of Truth 是什么？
4. 未来需要可搜索吗？
5. 未来需要 sync-aware 吗？
6. 未来需要关联到 Workspace 吗？
7. 需要在哪些 command surface 暴露？（CLI / TUI / Palette）

如果答不清楚，不要急着写代码。

## 7. 文档同步义务

实现重要功能或变更后，agent 必须更新相关文档：

| 变更类型 | 必须更新 |
|----------|----------|
| 产品层面 | `01_PRODUCT.md` / `02_DOMAIN.md` / `04_INTERACTION.md` |
| 架构层面 | `09_DESIGN.md` / `12_CONTRACTS.md` |
| 交付层面 | `11_PLAN.md` / `13_ACCEPTANCE.md` |
| 决策层面 | `01_PRODUCT.md` §Decision Log |

如果没有文档需要更新，agent 仍需解释为什么不需要。

## 8. 分阶段实施规则

如果必须分阶段实施：

1. 在文档和数据模型中保留完整产品形态
2. 交付一个较小的实现切片
3. 清楚标记哪些是 stub / 缺失 / 延后的
4. 不要假装较小的切片就是完整产品

分阶段是可接受的。偷偷缩小产品是不可接受的。

## 9. 编码风格

因为 owner 预期未来会有变更，agent 应该优化：

- 领域清晰分离
- 显式契约
- 强命名
- 隔离职责
- 可逆选择
- 未来维护者低惊讶

agent 应该避免：

- 隐藏耦合
- 隐式语义
- view 驱动业务逻辑
- 过度优化的抽象（让变更更难）

## 10. 完成标准

实现不是"完成"仅仅因为：

- 代码编译通过
- 命令存在
- 视图能渲染

实现是"完成"当且仅当：

- 行为正确
- 满足验收标准（`13_ACCEPTANCE.md`）
- 产品模型仍然连贯
- 未来扩展仍然可能

## 11. 汇报规则

汇报进度时，agent 必须说清：

- 改了什么
- 属于哪个产品域
- 文档是否已更新
- 还缺什么
- 剩余的变更风险

这一点尤其重要，因为 owner 不通过读代码来判断进度。

## 12. 冲突解决

当短期便利和长期产品连贯发生冲突时，agent 应该选择长期产品连贯，除非 owner 明确指示相反。
