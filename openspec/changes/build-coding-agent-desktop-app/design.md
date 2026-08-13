## Context

Klaude 从空仓库开始建设，目标是交付一个本地优先的 Coding Agent 桌面 MVP。参考架构已确定 Wails + Go + React/TypeScript 技术栈，并要求 React 不理解 Agent 内部状态、Agent 不依赖 Wails、Tool 不依赖 UI。首版必须覆盖真实编码闭环：打开项目、发起会话、流式运行 Agent、读写与搜索代码、执行命令、审批高风险操作、查看/撤销变更并恢复历史会话。

主要约束如下：

- 文件和进程操作发生在用户机器上，工作区边界与审批必须在后端强制执行，前端校验不能作为安全边界。
- 模型厂商的流式协议与工具调用格式会变化，核心 Runtime 不能依赖某个 SDK 的事件类型。
- Wails RPC 适合请求/响应，持续文本、工具进度和审批需要有序事件流。
- SQLite 是权威业务数据源；JSONL 仅用于可观测性、诊断与重放分析。
- MVP 使用逻辑隔离与路径限制，不具备 OS 级沙箱的安全承诺。
- MVP 依赖本机 `git` 和 `rg`，并需要在缺失时优雅降级或报告可操作错误。

## Goals / Non-Goals

**Goals:**

- 构建可在支持的桌面平台启动、打开本地 Git/非 Git 项目并恢复状态的 Wails 应用。
- 建立可独立单元测试和未来复用于 CLI/headless 场景的 Go Agent Runtime。
- 用统一 Provider、Tool 和 Event 契约完成流式文本与多轮工具调用闭环。
- 让每次有副作用的执行都经过路径校验、权限策略和必要的用户审批。
- 将一个用户请求建模为可追踪的 Agent Turn，并完整关联消息、工具调用、审批、用量和文件变更。
- 让用户能检查 Monaco Diff、取消运行并按 Turn 撤销由 Klaude 产生的文件修改。
- 用 SQLite、JSONL Trace 和结构化日志提供恢复、审计和问题诊断能力。

**Non-Goals:**

- 不在本变更实现 MCP、Skill、PTY/xterm 交互终端、浏览器或 Web 工具。
- 不实现多 Agent、远程 Agent、Git worktree、容器工作区或远程 Executor。
- 不实现 Tree-sitter、LSP、代码索引、语义检索或长期 Memory。
- 不实现自动上下文摘要/压缩；MVP 只做确定性预算、裁剪与明确报错。
- 不提供 OS/容器级强沙箱，也不允许 UI 将当前逻辑隔离描述为安全沙箱。
- 不保证任意 Provider；首版只交付一个 OpenAI 兼容 Provider，但保留适配接口。

## Decisions

### 1. 采用端口与适配器分层，并固定单向依赖

代码按 `frontend`、`internal/app`、`internal/agent`、领域接口与基础设施适配器拆分。Wails 仅绑定 `AppService`；它把用户请求委派给 Project、Session 和 Agent 管理器，并由 `EventBridge` 把内部事件转换成前端事件。Agent 只依赖 `ModelProvider`、`ContextManager`、`ToolRegistry`、`PermissionEngine`、`ApprovalManager`、`EventSink` 和存储接口。

选择该方案是为了让核心循环可以用 fake provider/tool 做确定性测试，也为以后加入 CLI 和其他桌面外壳留出空间。备选方案是把 Agent 直接写进 Wails binding；实现更快，但会把 UI 生命周期、模型调用和系统能力耦合在一起，因此不采用。

建议目录以参考架构为基准：

```text
cmd/klaude
frontend/src/{features,components,stores,hooks,lib,types}
internal/{app,agent,context,model,tool,executor,permission,approval,event}
internal/{project,session,storage,filesystem,search,git,config,trace}
migrations
```

### 2. RPC 负责命令，版本化 Typed Event 负责异步状态

前端通过 Wails RPC 调用 `OpenProject`、`CreateSession`、`SendMessage`、`CancelAgent`、`ResolveApproval`、`GetTurnChanges`、`UndoTurn` 和查询/配置接口。`SendMessage` 在用户消息与 Turn 成功落库后立即返回 `turnId`，后续结果通过事件送达。

内部事件使用稳定 envelope：

```text
version, eventId, sequence, occurredAt,
projectId, sessionId, turnId, type, payload
```

首版事件至少包含 `agent.started`、`agent.text_delta`、`agent.tool_started`、`agent.tool_finished`、`agent.approval_required`、`agent.usage`、`agent.error`、`agent.cancelled` 和 `agent.finished`。每个 Turn 的 `sequence` 单调递增；前端按 `eventId` 去重并按序归并，重新打开会话时以 SQLite 快照为准。

备选方案是轮询数据库，或让每个模块直接发 Wails 事件。轮询会损害流式体验，直接发事件会令领域层依赖桌面框架，因此不采用。

### 3. 将 Agent Turn 建模为显式状态机

一个 Session 同时最多有一个活动 Turn。状态流为：

```text
queued -> running -> waiting_approval -> running
                       |                 |
                       v                 v
                    cancelled       completed
                         \             /
                          -> failed <-
```

Agent Loop 在每轮构建上下文、调用 Provider、聚合文本/工具调用、执行工具并把结果追加回上下文，直到模型完成、取消、不可重试错误或达到最大轮次。所有长耗时接口接受同一个 `context.Context`；取消必须向 Provider stream、审批等待、Executor 和工具传播。模型一次返回多个调用时，只读且声明可并发的工具可并行；写工具、Shell 和同资源调用在 MVP 中串行执行。每个项目的变更锁阻止不同会话同时执行有副作用的工具。

备选方案是隐式 goroutine 状态和任意并行。它难以恢复、取消和避免同一工作区写冲突，因此不采用。

### 4. Provider Adapter 归一化厂商流事件

定义 `ModelProvider.Stream(ctx, ModelRequest) -> ModelEvent stream`。领域事件限定为文本增量、工具调用开始/增量/完成、用量和模型完成；Provider 负责解析 SSE、拼接工具参数、标准化错误与用量。首个适配器面向 OpenAI 兼容 API，使用独立配置的 endpoint/model；核心层不暴露厂商 SDK 类型。

凭据通过环境变量或平台安全凭据服务引用解析，SQLite、TOML、事件、Trace 和日志均不得写入密钥。请求元数据、Authorization header 和工具敏感参数在记录前经过集中脱敏。

选择 HTTP/协议适配器而非让 Runtime 直接依赖官方 SDK，是为了保持领域模型稳定并便于测试兼容 endpoint。代价是需要自行维护协议解析；实现时用契约测试覆盖正常流、拆分工具参数、限流、断线和取消。

### 5. 所有工具统一经过 Dispatcher 安全管线

工具实现统一的名称、描述、JSON Schema、元数据与 `Execute` 契约。调用路径固定为：

```text
schema validation
-> canonicalize target/workdir
-> workspace boundary validation
-> permission evaluation
-> approval when ASK
-> executor/tool execution
-> result normalization and size limiting
-> trace/persistence/event
```

内置工具包括 `read_file`、`write_file`、`apply_patch`、`list_directory`、`glob`、`grep`、`shell`、`git_status` 和 `git_diff`。文件工具使用 Go 文件 API；搜索优先调用 `rg`；Git 通过参数数组调用系统 `git`；Shell 仅执行非交互命令并返回 stdout、stderr、exit code、duration 与 truncation metadata。命令不通过未校验的拼接字符串传给 shell，除非平台适配层明确实现、展示并审批完整 shell 语义。

路径校验在解析 symlink 后验证真实目标仍位于项目根目录内。结果超过限制时保留头尾、错误摘要和原始 Trace 引用，避免耗尽模型上下文或前端内存。

备选方案是让模型只使用通用 Shell。结构化工具更便于权限控制、追踪、撤销和跨平台适配，因此不采用纯 Shell 方案。

### 6. Permission、Approval 与 Sandbox 保持三个独立概念

`PermissionEngine` 对规范化请求返回 `ALLOW`、`ASK` 或 `DENY`。默认策略是工作区内只读工具允许，写工具与 Shell 询问，越界路径和明确禁止的命令拒绝。`ASK` 产生包含工具名、规范化参数、工作目录、风险与人类可读说明的待处理 Approval；仅匹配当前 approval id 与调用摘要的 `allow_once` 才能继续，`reject` 返回结构化工具结果给 Agent。审批期间仍可取消 Turn。

MVP 的 `LocalExecutor` 只提供工作目录限制、显式 argv、环境白名单、超时、输出上限和进程树取消。这是逻辑隔离，不标记为 OS Sandbox。平台沙箱或容器以后通过 `Executor`/`Sandbox` 适配器加入。

备选方案是默认信任模型或只在 UI 隐藏危险按钮；两者都无法构成后端执行边界，因此不采用。

### 7. 文件写入采用直接修改、写前快照和 Turn changeset

`write_file` 与 `apply_patch` 在原子替换前读取当前内容、校验预期基线并保存内容寻址快照；成功后记录 before/after hash、统一 diff、行数统计和文件状态。`apply_patch` 在上下文不匹配时失败，不做模糊写入。Turn changeset 聚合该 Turn 的全部 FileChange，UI 用 Monaco Diff Editor 展示。

`UndoTurn` 仅在当前文件 hash 仍等于该 Turn 的 after hash 时自动恢复 before snapshot；若用户或其他进程已继续修改，系统必须拒绝覆盖并展示冲突。新建文件可在 hash 匹配时删除，删除文件（MVP 无 delete tool）不在范围内。

选择直接修改是为了让编译器、测试和 Git 立即看到修改。备选虚拟 staging workspace 更安全，但会显著增加路径映射、命令执行和合并复杂度，留待后续。

### 8. SQLite 为恢复真相源，Trace 与日志承担可观测性

SQLite 使用内嵌、顺序编号且事务化的 migrations。核心表至少包含 `projects`、`sessions`、`messages`、`agent_turns`、`tool_calls`、`tool_results`、`approvals`、`file_changes`、`usage` 和 `settings`。用户消息与新 Turn 同事务创建；工具开始/完成、审批状态和终态都及时持久化。启动时遗留的 `running`/`waiting_approval` Turn 标记为 `interrupted`，允许用户查看历史但不自动重放副作用。

JSONL Trace 按 session/turn 记录脱敏的模型与工具生命周期，结构化日志使用 `slog`。Trace 不是恢复来源，损坏或缺失不得阻止应用启动。本地数据位于平台用户数据目录下的 Klaude 命名空间；展示上的 `~/.klaude/` 是兼容约定，实际路径由平台适配器解析。

备选方案是只写 JSONL；它不适合事务查询、关系关联和并发更新，因此不采用。

### 9. 配置按层合并，项目内容不能静默提升权限

配置优先级为应用默认值、用户配置、项目配置、会话选择。项目可提供 `.klaude/config.toml`、`.klaude/instructions.md`，并可读取 `AGENTS.md`；这些内容可以调整模型上下文与非敏感偏好，但不得覆盖用户级拒绝规则、工作区边界或凭据来源。解析失败应报告文件与字段并回退到最后一个有效配置，而不是带着部分未知配置执行。

敏感值只保存引用（例如环境变量名或系统凭据条目 id）。UI 设置保存前执行 schema 校验并清除日志中的敏感字段。

### 10. Context Builder 采用确定性优先级和硬预算

MVP 上下文优先级为 System/App 指令、项目指令、已启用的会话设置、最近对话、工具结果和显式读取的代码。Context Builder 为输出预留固定预算，对大工具结果先结构化截断；若必需内容仍超过模型窗口，Turn 以可操作错误停止，不进行隐式丢失关键信息的摘要。

这比首版引入 LLM compaction 更可预测，且便于测试。自动摘要、长期 Memory 与语义检索将在后续能力中设计。

### 11. 前端以服务端状态为准并隔离功能状态

React 使用 feature 目录与 Zustand store 管理 project、session、chat、agent、changes、approval 和 settings。初次加载与重连先查询后端快照，再订阅增量事件。三栏布局中左侧为项目/会话/文件，中间为对话与输入，右侧为变更/Diff；窄窗口允许面板折叠，但审批必须保持可发现且不能被后台自动确认。

Markdown 渲染默认禁用原始 HTML，代码块使用 Shiki，Diff 使用 Monaco。来自项目、模型与工具的文本一律视为不可信展示内容。

### 12. 测试边界以高风险闭环为中心

Go 单元测试使用 fake Provider、in-memory EventSink、临时工作区和临时 SQLite 覆盖 Agent 状态机、路径/symlink 逃逸、审批绑定、取消传播、工具结果截断、快照与冲突撤销。Provider 使用录制的协议 fixture 做契约测试。前端组件测试覆盖事件归并、流式文本、审批和 Diff 状态；Wails 端到端冒烟测试覆盖打开临时项目、运行 fake Agent、批准补丁、查看 Diff、撤销和重启恢复。

## Risks / Trade-offs

- [逻辑隔离无法抵御恶意进程或系统级攻击] → UI 明确标注安全边界，默认询问写/Shell，限制路径、argv、环境与超时，并把强沙箱列为后续发布能力。
- [直接修改可能与用户同时编辑冲突] → 写入前校验基线，使用原子替换和 after hash，撤销冲突时拒绝覆盖。
- [OpenAI 兼容服务的流协议存在差异] → 将解析封装在 Provider 内，用 fixture 契约测试和能力检测隔离差异。
- [事件丢失、重复或乱序导致 UI 状态漂移] → 使用 event id/sequence、前端幂等归并，并允许随时从 SQLite 快照重建。
- [SQLite 写入与高频 delta 造成性能压力] → 文本 delta 在内存中短暂合并后批量落库，关键状态转换仍同步提交；启用 WAL 与合理 busy timeout。
- [Shell/Git/rg 跨平台差异] → 使用平台适配器、显式 argv 和 capability probe；缺失依赖时返回安装提示，不静默换成不安全路径。
- [上下文无自动压缩导致长会话提前到达上限] → 显示上下文用量、确定性裁剪非关键结果，并给出新建会话/后续 compaction 提示。
- [工件范围较大导致首版交付周期膨胀] → 按任务中的垂直切片推进，先用 fake Provider 跑通安全闭环，再接真实 Provider 和完善 UI。

## Migration Plan

1. 初始化 Wails/Go/React 工程、CI 与模块边界测试，保持桌面壳可启动。
2. 添加 SQLite v1 migration 与 repositories；全新安装创建数据库，已有但无 schema 的开发目录先备份再迁移。
3. 以 fake Provider 建立 Project/Session、EventBus、Agent 状态机和前端事件闭环。
4. 加入结构化只读工具，再加入 Permission/Approval、写工具、快照、Diff 和 Undo。
5. 接入 OpenAI 兼容 Provider、凭据解析、用量与错误归一化。
6. 完成启动恢复、Trace/日志脱敏、跨平台 capability probe 和端到端验收后发布 MVP。

回滚应用版本时不得自动降级数据库。每个 migration 在变更 schema 前制作数据库备份；若启动迁移失败，应用进入只读诊断模式并保留原数据库。工作区文件不通过数据库 migration 回滚，用户文件只能通过已记录且 hash 匹配的 Turn snapshot 撤销。

## Open Questions

- 首个正式支持的平台组合与 Wails WebView 最低版本需要在实现开始时由发布目标确认；架构默认 macOS、Windows、Linux 均保持可编译。
- 平台安全凭据服务在三平台的具体库需要通过维护状态、签名体积和无 GUI 测试能力评估；若某平台暂不可用，只允许环境变量引用，不回退到明文存储。
- OpenAI 兼容 endpoint 的首版工具调用兼容矩阵需要在 Provider 契约测试阶段固化。
