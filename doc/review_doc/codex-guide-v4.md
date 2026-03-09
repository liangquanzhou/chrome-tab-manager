# Codex Guide V4

## 主题

**先把 daemon contract 和 CLI 路径做稳**

这份文档是给 Claude 的专项执行指引，不是泛泛而谈的建议。

目标只有一个：

> **把 CTM 的 daemon contract 和 CLI 路径收口成“可定义、可测试、可发布、可长期扩展”的稳定层。**

---

## 一句话判断

如果当前 CTM 只能优先稳一层，那一层应该是：

> **daemon contract + CLI**

而不是先继续强化 TUI。

原因很直接：

- daemon contract 是系统真相
- CLI 是最薄、最可测试、最可复现的用户入口
- TUI 是长生命周期状态机，天然更适合做交互层，而不是做真相层

换句话说：

- **daemon contract** 决定系统是否一致
- **CLI** 决定系统是否可验证
- **TUI** 决定系统是否好用

顺序不能反。

---

## 为什么这件事要优先于 TUI

当前代码结构已经很说明问题了：

- `internal/protocol/messages.go` 已经定义了统一消息模型和错误码
- `doc/12_CONTRACTS.md` 已经开始承担 API contract 文档角色
- CLI 大多数命令都走 `cmd/helpers.go` 里的 `connectAndRequest()`
- TUI 则是一个 2500+ 行的长生命周期、事件驱动、状态驱动程序

这意味着：

### 1. CLI 更接近“可执行的 contract”

CLI 的模型很简单：

1. 组 payload
2. 发 request
3. 收 response
4. 输出结果

所以 CLI 非常适合承担：

- 精确复现
- 自动化测试
- JSON 稳定输出
- 错误语义校验
- 回归保护

### 2. TUI 更适合看现场，不适合当真相层

TUI 的问题不是“写得不好”，而是它天然复杂：

- 有多 view
- 有多 mode
- 有异步 `tea.Cmd`
- 有 event subscription
- 有 preview cache
- 有 target auto-recover
- 有 keyboard / mouse 分支

所以 TUI 非常适合：

- 人工观察
- 交互排查
- 状态探索

但它天然不适合先承担“定义 contract 正确性”的职责。

### 3. 如果 daemon contract 和 CLI 先稳，TUI bug 会降级成“表面 bug”

这是最重要的工程收益。

一旦 daemon contract 和 CLI 已经稳定：

- TUI 出问题时，先用 CLI 验证 contract
- 如果 CLI 正常，说明问题在 TUI 表面层
- 如果 CLI 也异常，说明问题在 daemon / contract 层

这会极大降低定位成本。

---

## 当前仓库已经具备的基础

当前 CTM 并不是从零开始做这件事，很多基础已经有了。

### 1. 协议模型已经存在

`internal/protocol/messages.go` 已经定义了：

- `Message`
- `MessageType`
- `TargetSelector`
- `ErrorCode`

这很好，说明系统已经有统一 envelope。

### 2. contract 文档已经存在

`doc/12_CONTRACTS.md` 不只是存在，而且已经开始承担更强角色：

- 文档标题已明确标成 authoritative
- 开头已经定义了 selector / `--json` / exit code / error code 约定
- 多数核心 action 已经从“只有示例 JSON”往“带 layer/target/CLI 属性”迁移

这说明“文档化 contract”这件事已经开了头，不需要另起炉灶。

### 3. CLI 已有统一请求入口

`cmd/helpers.go` 的 `connectAndRequest()` 目前承担了 CLI 的公共请求链路：

- connect
- timeout
- daemon auto-start fallback
- request

这非常重要，因为它给“统一 CLI 语义”提供了天然收口点。

### 4. daemon 已经有清晰的 action 分流结构

`internal/daemon/hub.go` 已经按 action prefix 分流：

- `targets.*`
- `sessions.*`
- `collections.*`
- `bookmarks.*`
- `search.*`
- `workspace.*`
- `sync.*`
- `tabs.*` / `groups.*` / `windows.*` / `history.*` / `downloads.*`

这说明 daemon 侧也有天然的 contract 收口位置。

---

## 当前真正不稳的点

这部分最关键。不是说“要更规范一点”，而是要说清楚现在到底哪里不稳。

## 1. contract 文档已经进入 authoritative 形态，但还没完全和实现对齐

这块和上一轮相比，已经明显前进了。

问题不再是“完全没有 authoritative contract”，而是：

> **文档已经开始扮演唯一真相源，但覆盖和实现对齐还没有完全闭环。**

主要问题通常会出现在这些维度：

- 某些已文档化 action 仍有实现偏差
  - 例如文档开头写 daemon 也接受 `channel` / `label` selector，但当前 `resolveTarget()` 实际只按 `targetId` / default target / single-target fallback 解析
- contract inventory 这轮已经比之前完整很多，但仍需要继续做“文档条目是否真实对齐实现”的复核，而不只是补 action 名单
- 某些 hybrid action 虽然已有行为说明，但 partial failure / best-effort / ordering 仍值得继续收紧

这会导致一个问题：

> 文档已经开始约束实现，但现在还不能假设“写在文档里的都已经严格落地”。

consolidation 阶段下一步要做的，是把这份 contract 从“强意图文档”推进成“强约束文档”。

## 2. daemon 错误语义还需要继续收口

这块也比上一轮更好了。

`targetErrorCode(err)` 相关逻辑已经抽到独立的 `internal/daemon/errors.go`，并且开始使用 sentinel / typed error + `errors.Is()` 归类。

这意味着“target 解析错误不该再靠 message wording 判断”这件事，已经开始落地，不需要从零建议。

当前真正剩下的工作，是把这种明确性继续扩展到更广的 daemon error surface，而不是只停留在 target resolution 这一段。

更准确的下一步应该是：

- 继续用 typed error / sentinel error 扩展关键错误路径
- 保证相同失败语义始终映射到相同 protocol error code
- 把“typed mapping 已落地的范围”和“仍待收口的范围”说清楚

## 3. CLI 当前是统一入口，但还不是统一规范

`connectAndRequest()` 已经把链路收了一部分，而且 `12_CONTRACTS.md` 与 `cmd/root.go` / `protocol.ProtocolError` 现在也已经开始把 CLI 约定接到代码里。

当前真正的问题不再是“完全没有规范”，而是：

> **规范已经部分写下并接入实现了，但还需要被系统性验证和统一贯彻。**

具体还要继续收口的点包括：

- `--json` 是否在所有支持命令上都稳定输出 `payload` JSON
- human output 和 JSON output 是否严格分离
- exit code 约定虽然已经接入主错误路径，但是否覆盖所有关键命令分支、并有足够测试保护
- auto-start daemon 的策略是否适用于所有命令
- `--target` 是否就是唯一官方 selector，还是未来要支持 `channel/label`

如果这些不定死，CLI 看起来统一，实际上只是在“共享 helper”而不是“共享 contract”。

## 4. 当前 target selector 的产品面和协议面还不完全一致

协议里的 `TargetSelector` 模型声明了：

- `targetId`
- `channel`
- `label`

但当前实现里：

- daemon 的 `resolveTarget()` 实际只按 `targetId` / default target / single-target fallback 解析
- CLI 现在实际暴露的也只有 `--target`

这不一定是 bug，但必须做选择：

- 要么明确：当前系统正式只支持 `targetId`
- 要么把 daemon 和 CLI 一起补齐 `channel` / `label`

最怕的是处在中间态：

- 协议宣称支持
- daemon/CLI 并未真正实现或不公开支持
- 文档又没有明确限定

## 5. doctor / install-check 已进入 health check，但还不是完整链路诊断

这块也已经比上一轮前进了。

`doctor` 现在已经不只是 `install --check` 的 alias，而是开始做更真实的 runtime health check：

- binary / manifest / LaunchAgent 存在性
- daemon socket reachability
- `targets.list` request/response
- extension connection count

所以当前更准确的判断不是“它只是文件存在性检查”，而是：

> **它已经进入最小运行健康检查阶段，但还没有覆盖到完整链路校验。**

还没完全回答的问题仍然包括：

- daemon 是否真能起来
- extension pairing 是否真匹配
- manifest 内容是否和当前 binary / extension id 一致
- 除 `targets.list` 之外的 extension action 是否可用
- install / doctor / first real action 之间是否构成完整 release path

所以下一步不是“从 alias 变成 doctor”，而是“从最小 health check 变成更可靠的链路诊断”。

---

## 要把 daemon contract 做稳，具体怎么做

这一部分是给 Claude 的执行重点。

## A. 把 contract 从“说明文档”提升为“实现约束”

每个 action 都必须有一份完整 contract，至少包含下面这些字段：

### 每个 action 必须明确的 9 项

1. **Action 名**
2. **Layer 类型**
   - local
   - forward
   - hybrid
3. **Target 要求**
   - required
   - optional
   - disallowed
4. **Request payload shape**
5. **Response payload shape**
6. **Stable error codes**
7. **Partial failure 语义**
8. **Idempotency / retry 语义**
9. **CLI exposure**
   - supported
   - partial
   - internal only

### 建议直接采用统一模板

```md
## action.name

- Layer: local | forward | hybrid
- Target: required | optional | disallowed
- CLI: supported | partial | internal only
- Request:
  - fieldA: type, required, meaning
- Response:
  - fieldB: type, meaning
- Errors:
  - INVALID_PAYLOAD: ...
  - TARGET_OFFLINE: ...
  - TARGET_AMBIGUOUS: ...
- Partial Failure:
  - ...
- Idempotency:
  - ...
- Notes:
  - ...
```

### Claude 的工作要求

- 不要再只写 JSON 示例
- 要把错误集合、target 语义、partial failure 语义补齐
- 对 hybrid action（如 save/restore/switch）尤其要写清顺序和 best-effort 边界

## B. 在代码里建立 action metadata，而不是只靠 switch 分支

当前按 prefix + switch 分流没有错，但对于“稳定 contract”来说还不够强。

建议引入一个轻量 action registry，至少记录：

- action 名
- 是否需要 target
- 是 local / forward / hybrid
- 允许的错误码集合
- 是否有 CLI 入口
- 是否是正式支持能力

这个 registry 不一定要复杂到自动生成全部代码，但它至少应该成为：

- 文档校对依据
- CLI 暴露校对依据
- 测试覆盖校对依据

这样以后 review 一个 action，不用再在多个文件里人工拼装它的真实状态。

## C. 把错误分类改成 typed mapping

`targetErrorCode(err)` 这种字符串归类逻辑，建议逐步替换成 typed error。

推荐方式：

- 定义 sentinel / typed errors
  - `ErrNoTargetsConnected`
  - `ErrTargetNotFound`
  - `ErrMultipleTargets`
- `resolveTarget()` 返回 typed error
- `forwardToExtension()` 和 handler 只负责 error code mapping

目标是：

> 相同语义的失败，始终产出相同 error code，不依赖 message wording。

## D. 明确“forwarded action”的 contract 边界

forwarded action 最容易产生灰区。

必须明确三件事：

1. daemon 是否只是透明转发
2. daemon 是否会补充 / 约束 payload
3. daemon 是否会变换错误语义

例如：

- `tabs.list` 更接近 forward
- `sessions.restore` 是 hybrid
- `workspace.switch` 是 orchestrated hybrid

这三类 action 不应该在 contract 上混写。

建议 Claude 按这三档重新梳理：

- **Forward**
  daemon 不改业务语义，只负责 target 路由、timeout、错误包装
- **Hybrid**
  daemon 组合多个 action，对顺序和部分失败负责
- **Local**
  daemon 完全负责业务和持久化

---

## 要把 CLI 路径做稳，具体怎么做

## A. 把 CLI 定义成“contract 的可执行外壳”

CLI 不应该承担“重新发明业务语义”的职责。

CLI 应该只做四件事：

1. 参数解析
2. payload 组装
3. 调用 daemon
4. 结果渲染

如果某个 CLI 命令开始承担过多业务判断，长期就会和 daemon 语义漂移。

### Claude 的实现原则

- 业务语义放 daemon
- 参数语义放 CLI
- 输出语义放统一 renderer

## B. 统一 CLI 的四类语义

这四类必须定死，不要每个命令自己发挥。

### 1. Target 选择语义

必须明确：

- 当前 CLI 是否只支持 `--target`
- 是否要新增 `--channel` / `--label`
- 多 selector 是否允许同时使用
- 默认 target / 单 target fallback / ambiguous target 的行为是否一致

如果短期不打算支持 `channel/label`，那就要在 contract 和 CLI 文档里明确写：

> 当前 CLI 正式支持的 selector 只有 `targetId`

不要让协议层能力误导产品层预期。

### 2. 输出语义

建议定死：

- `stdout`
  - 成功时输出结果
- `stderr`
  - 失败时输出错误
- `--json`
  - 一律输出稳定 JSON
  - 不混入 human text

并且要明确：

- `--json` 输出 payload 还是完整 envelope

我建议当前阶段继续保持 **payload-only JSON**，因为更适合脚本使用；但要文档化，并保证所有支持 `--json` 的命令行为一致。

### 3. exit code 语义

当前如果不定义 exit code，CLI 稳定性永远差一层。

建议至少约定：

- `0`: success
- `1`: usage / validation error
- `2`: daemon unavailable / install invalid
- `3`: target resolution error
- `4`: action execution error / timeout / chrome api failure

不一定一次全做完，但必须先定出规则。

### 4. auto-start 语义

`connectAndRequest()` 现在会在连接失败时尝试 auto-start daemon。

这很好，但也要回答：

- 哪些命令允许 auto-start
- 哪些命令不应该隐式拉起 daemon
- auto-start 失败时如何报错
- doctor / install-check 是否应该禁止隐式自启动

建议：

- 普通功能命令可以 auto-start
- 诊断类命令要明确显示链路状态，不要用 auto-start 掩盖问题

## C. 用统一 renderer，减少命令各自输出漂移

当前很多命令已经共享 `printJSON()`，但 human output 仍然分散在各命令里。

建议下一步建立轻量输出规范：

- list 类：表格输出
- get 类：详情输出
- mutate 类：确认输出
- `--json`：payload-only JSON

重点不是做复杂框架，而是保证同类命令的输出风格和字段命名一致。

## D. CLI 不要抢 daemon 的职责

有些事情一旦放进 CLI，很快就会形成双重语义源。

应该避免 CLI 自己定义：

- target fallback 规则
- partial failure 规则
- restore / switch 顺序语义
- 本地对象 schema

这些都应该只由 daemon contract 定义，CLI 只消费。

---

## 测试应该怎么补，才算真正“做稳”

只加一些 happy-path test，不算稳。

要把 daemon contract 和 CLI 稳下来，测试至少要分四层。

## 1. Contract tests

每个正式暴露 action 至少要覆盖：

- success
- invalid payload
- no target connected
- target ambiguous
- target offline
- extension timeout

不是所有 action 都需要全部六类，但至少要明确“适用 / 不适用”。

### 关键要求

- 测的是 error code，不只是 error message
- 测的是 response shape，不只是“返回成功”
- hybrid action 要测 partial failure 统计字段

## 2. CLI tests

CLI tests 不应该只测“命令跑完了”。

必须测：

- 参数解析
- JSON 输出 shape
- human output 关键字段
- stderr / stdout 分离
- exit code

如果一个命令支持 `--json`，就至少要有一条 JSON shape test。

## 3. Integration smoke tests

至少保留一组很小但真实的链路 smoke：

- daemon 起停
- CLI auto-start
- `targets.list`
- `tabs.list`
- 一个本地对象 action（如 `sessions.list`）

目的不是测全功能，而是保证“最小系统”始终成立。

## 4. Release smoke tests

这一层经常被忽视，但如果你要把 CLI 当正式入口，就必须补。

至少包括：

- `ctm install`
- `ctm install --check`
- `ctm doctor`
- daemon 可连接
- 一条只依赖 local 的 action
- 一条依赖 extension 的 action

---

## 给 Claude 的分阶段执行建议

不要一口气做大改，按下面顺序推进。

## Phase A: 定义真相

目标：让 contract 先可被引用。

Claude 要做：

1. 逐 action 复核并补齐 `doc/12_CONTRACTS.md`
2. 优先校正 contract 与实现已经不一致的条目
   - 例如 selector 约定里对 `channel` / `label` 的表述
3. 对每个 action 明确 target / error / partial failure / idempotency
4. 明确当前系统正式支持的 selector 范围
5. 核对 `payload-only JSON` / exit code 等 CLI 约定是否已被实现真实兑现

验收：

- `12_CONTRACTS.md` 可单独回答“这个 action 的稳定语义是什么”

## Phase B: 收口代码语义

目标：让实现和文档一致。

Claude 要做：

1. 把已存在的 typed error 机制继续扩展到更多关键错误路径
2. 梳理 forwarded / hybrid / local 三类 action
3. 统一 daemon 错误到稳定 error code
4. 审核 `connectAndRequest()` 的 auto-start / timeout / error surface

验收：

- 相同失败语义稳定产出相同 code
- action 边界不再靠读实现猜

## Phase C: 统一 CLI 行为

目标：让 CLI 成为可执行 spec。

Claude 要做：

1. 统一 `--json` 语义
2. 统一 stdout / stderr 规则
3. 统一 exit code 规则
4. 统一 target selector 规则
5. 给关键命令补 JSON shape tests

验收：

- 同类命令输出风格一致
- 脚本调用不会因为格式漂移而断

## Phase D: 建 release gate

目标：让这层真正可发布。

Claude 要做：

1. 增强 `doctor` / `install --check`
2. 建最小 smoke path
3. 把 release 路径写进文档
4. 让 CI 至少卡住 contract / CLI 回归

验收：

- 新机器上能明确判断是安装问题、连接问题、target 问题还是 action 问题

---

## Definition of Done

当且仅当满足下面条件时，才可以说“daemon contract 和 CLI 路径已经做稳”：

1. `doc/12_CONTRACTS.md` 不仅自称 authoritative，而且其关键条目已和实现对齐
2. 每个正式暴露 action 都有稳定的 request / response / error 定义
3. daemon 的关键错误分类不再依赖 message wording，且这种约束不只停留在 target resolution
4. CLI 的 target、JSON、stderr/stdout、exit code 规则一致
5. 每个正式 CLI action 都有至少一条 JSON 或错误语义测试，尤其覆盖 exit code / stdout / stderr 这些契约点
6. `doctor` / `install --check` 至少能覆盖从安装状态到最小运行链路的诊断，而不仅停留在当前最小 health check
7. TUI 再出问题时，可以先用 CLI 在同一 contract 上复现和切割问题

---

## 最后给 Claude 的一句话

如果只能抓一件事，就抓这句：

> **把 daemon contract 写成唯一真相，把 CLI 做成可执行规格，然后再让 TUI 去消费这套稳定语义。**

当前 CTM 已经不缺功能原型了。  
下一阶段真正缺的，是一条足够稳的底座，让所有上层行为都建立在同一份 contract 之上。
