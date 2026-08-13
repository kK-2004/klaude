## ADDED Requirements

### Requirement: Tools are registered with schemas and safety metadata
每个工具 SHALL 提供唯一名称、描述、JSON 参数 Schema 以及 readOnly、destructive、concurrent、requiresApproval 元数据；Dispatcher MUST 在执行前验证名称与参数。

#### Scenario: Dispatch a valid tool call
- **WHEN** 模型请求已注册工具且参数满足其 JSON Schema
- **THEN** Dispatcher SHALL 使用规范化参数进入安全执行管线

#### Scenario: Dispatch an unknown or invalid tool call
- **WHEN** 工具不存在或参数不满足 Schema
- **THEN** Dispatcher MUST 不执行任何系统操作并 SHALL 返回结构化验证错误

### Requirement: All filesystem targets remain inside the project workspace
文件与搜索工具 MUST 将相对路径解析到当前项目规范根路径内，并 MUST 在跟随现有 symlink 后再次验证边界。

#### Scenario: Access a file inside the workspace
- **WHEN** 工具参数解析为项目根目录内的可访问文件
- **THEN** 工具 SHALL 只对该规范目标执行请求的操作

#### Scenario: Attempt path traversal
- **WHEN** 路径包含遍历片段、绝对路径或 symlink 并最终解析到项目外
- **THEN** 工具 MUST 在读取或写入前拒绝调用并返回 workspace_boundary 错误

### Requirement: Structured file and search tools are available
系统 SHALL 提供 `read_file`、`list_directory`、`glob` 和 `grep`，支持受限范围与结构化结果，并 MUST NOT 要求 Agent 通过 Shell 完成基本文件读取和搜索。

#### Scenario: Read a line range
- **WHEN** `read_file` 收到项目内文件与有效行范围
- **THEN** 工具 SHALL 返回带行号、编码信息和是否截断的内容

#### Scenario: Search source text
- **WHEN** `grep` 收到有效模式与项目内范围且 `rg` 可用
- **THEN** 工具 SHALL 返回按文件和行号组织的匹配并遵守结果上限

#### Scenario: Search dependency is missing
- **WHEN** `grep` 或 `glob` 需要 `rg` 但系统未找到可执行文件
- **THEN** 工具 SHALL 返回 dependency_unavailable 与安装提示且 MUST NOT 静默改用不受控 Shell

### Requirement: Write and patch operations validate their baseline
系统 SHALL 提供 `write_file` 与 `apply_patch`；每次修改 MUST 在审批后、写入前验证预期基线并使用安全替换，补丁上下文不匹配时 MUST 不写入。

#### Scenario: Apply a matching patch
- **WHEN** 已批准补丁的预期上下文与当前文件一致
- **THEN** 工具 SHALL 原子写入目标并返回关联 FileChange

#### Scenario: File changed after patch was proposed
- **WHEN** 当前文件 hash 或补丁上下文与工具调用基线不一致
- **THEN** 工具 MUST 拒绝写入并返回 stale_baseline 冲突

#### Scenario: Write fails partway
- **WHEN** 临时写入、权限设置或原子替换失败
- **THEN** 工具 MUST 保留原文件且 MUST NOT 报告成功 FileChange

### Requirement: Shell execution is non-interactive and bounded
`shell` 工具 SHALL 通过 LocalExecutor 运行非交互命令，使用明确工作目录、参数、环境白名单、超时和输出上限，并 MUST 支持取消后终止所属进程树。

#### Scenario: Command succeeds
- **WHEN** 已批准命令在超时前以零状态退出
- **THEN** 工具 SHALL 返回 stdout、stderr、exitCode、duration 和 truncation metadata

#### Scenario: Command exits non-zero
- **WHEN** 子进程正常结束但 exit code 非零
- **THEN** 工具 SHALL 返回非零 exit code 作为工具结果且 MUST NOT 将其伪装成 Runtime 崩溃

#### Scenario: Command times out
- **WHEN** 命令超过批准的 timeout
- **THEN** Executor MUST 终止进程树并返回 timeout 结果

### Requirement: Git status and diff use the system Git client safely
系统 SHALL 提供 `git_status` 与 `git_diff`，通过显式参数数组在项目 Git root 内调用系统 `git`，并 MUST 兼容非 Git 项目和缺失 Git 的错误情况。

#### Scenario: Inspect a Git project
- **WHEN** 当前项目属于 Git 工作树且系统 Git 可用
- **THEN** 工具 SHALL 返回结构化状态或 diff，不修改仓库

#### Scenario: Inspect a non-Git project
- **WHEN** 当前项目不属于 Git 工作树
- **THEN** 工具 SHALL 返回 not_a_repository 而不影响其他项目能力

### Requirement: Tool results are bounded and traceable
每个 ToolResult SHALL 包含内容、成功状态、错误代码、truncated、metadata 和可选 rawRef；超出配置上限的输出 MUST 被安全裁剪，而完整脱敏输出 SHALL 仅通过 Trace 引用保留。

#### Scenario: Tool produces oversized output
- **WHEN** stdout、搜索结果或文件内容超过结果上限
- **THEN** 工具 SHALL 返回保留关键头尾与错误摘要的受限内容、设置 truncated，并提供可解析 rawRef
