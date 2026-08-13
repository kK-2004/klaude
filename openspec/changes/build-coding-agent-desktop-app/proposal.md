## Why

开发者需要一个本地优先、可审查且可控的桌面 Coding Agent，把项目浏览、Agent 对话、代码修改、命令执行与 Git 变更检查统一到一个应用中。当前仓库尚无实现，因此现在需要先建立一个边界清晰的 MVP 契约，为后续扩展多模型、Skill、MCP、PTY 与强沙箱保留稳定基础。

## What Changes

- 创建基于 Wails、Go、React 和 TypeScript 的 Klaude 桌面应用壳，提供项目、会话、对话和变更三栏工作区。
- 支持打开本地项目、创建和恢复会话，并持久化消息、Agent Turn、工具调用、审批与文件变更。
- 实现与 Wails 解耦的单 Agent Runtime，支持上下文构建、LLM 流式输出、工具调用循环、取消、错误与完成事件。
- 建立统一 Model Provider 接口并提供首个 OpenAI 兼容 Provider；模型凭据不得明文写入项目配置或数据库。
- 提供结构化内置编码工具：文件读取/写入/补丁、目录/Glob/Grep 搜索、非交互 Shell，以及 Git status/diff。
- 在所有有副作用的能力前执行工作区边界校验、权限判定和用户审批；MVP 提供逻辑隔离，不宣称具备 OS 级强沙箱。
- 在写入前保存快照，记录每个 Agent Turn 的 changeset，并在 Monaco Diff Editor 中展示可审查的变更。
- 通过内部 Typed Event Bus 将运行事件扇出到持久化、JSONL Trace 与 Wails Event Bridge，使 Agent、Tool 与 UI 保持解耦。
- 明确 MVP 不包含 MCP、Skill、PTY 终端、复杂上下文压缩、多 Agent、代码索引/LSP、远程执行和 OS/容器级沙箱。

## Capabilities

### New Capabilities

- `desktop-workspace`: 桌面应用启动、本地项目打开与三栏工作区导航体验。
- `session-conversation`: 项目内会话生命周期、消息提交、流式对话展示、恢复与取消。
- `agent-runtime`: 与桌面框架解耦的 Agent Loop、上下文构建、工具调用和 Typed Event 生命周期。
- `model-provider`: 统一模型流式接口、OpenAI 兼容 Provider、模型选择与安全凭据解析。
- `coding-tools`: 受工作区约束的文件、搜索、补丁、Shell 与 Git 工具及结构化结果。
- `permission-approval`: ALLOW/ASK/DENY 策略、审批交互、风险信息和执行前强制检查。
- `change-review`: 写前快照、按 Agent Turn 聚合的文件变更、Diff 查看及安全撤销。
- `local-persistence`: SQLite 主数据、配置分层、JSONL Trace、结构化日志和启动恢复。

### Modified Capabilities

无。

## Impact

- 新增 Wails/Go 后端、React/TypeScript 前端、SQLite migrations、应用配置与构建脚本；仓库将从空项目变为可运行的跨平台桌面应用。
- 引入 Wails、React、TypeScript、Vite、Tailwind CSS、Zustand、Monaco Editor、SQLite 驱动以及 OpenAI API 客户端或兼容 HTTP 适配层。
- 依赖本机 `git` 与 `rg` 执行 Git 和代码搜索；缺失时须向 UI 返回可操作的能力错误。
- 新增前后端 RPC 与事件契约、数据库 schema、本地 `~/.klaude/` 数据布局，以及项目级 `.klaude/` 配置/指令读取约定。
- 文件写入和命令执行会影响用户打开的工作区，因此路径校验、权限审批、快照和取消语义属于发布阻断要求。
