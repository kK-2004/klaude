## ADDED Requirements

### Requirement: SQLite is the authoritative local state store
系统 SHALL 使用 SQLite 持久化 projects、sessions、messages、agent_turns、tool_calls、tool_results、approvals、file_changes、usage 与 settings，并 MUST 通过外键或等价约束保持关联完整性。

#### Scenario: Restart after completed work
- **WHEN** 用户在 Turn 完成后关闭并重新启动应用
- **THEN** 系统 SHALL 从 SQLite 恢复项目、会话、消息、工具历史、用量与变更记录

#### Scenario: Trace files are missing
- **WHEN** SQLite 完整但 JSONL Trace 被删除或损坏
- **THEN** 系统 SHALL 仍能恢复业务状态并把 Trace 标记为不可用

### Requirement: Critical lifecycle writes are transactional and durable
系统 MUST 在同一事务中创建用户消息与 Agent Turn，并 SHALL 在工具、审批和 Turn 的关键状态转换后持久化对应记录；事务失败时 MUST 不暴露未提交成功状态。

#### Scenario: Create a Turn transactionally
- **WHEN** 用户消息插入成功但 Turn 插入失败
- **THEN** 系统 MUST 回滚用户消息且 MUST 不启动 Agent

#### Scenario: Persist a tool completion
- **WHEN** 工具结束并产生结果或 FileChange
- **THEN** 系统 SHALL 在发出完成事件前持久化完成状态与关联记录

### Requirement: Startup recovery never replays side effects automatically
启动时系统 SHALL 把遗留 running 或 waiting_approval Turn 标记为 interrupted，把 pending Approval 标记为 expired，并 MUST NOT 自动重发模型请求、重新执行工具或应用补丁。

#### Scenario: Recover after process termination
- **WHEN** 应用在 Shell 或文件审批期间异常终止后重启
- **THEN** 系统 SHALL 展示 interrupted 历史与已确认发生的变更，并要求用户显式发起新操作

### Requirement: Database schema changes use ordered migrations
系统 SHALL 使用内嵌、版本化、顺序执行且事务化的 migrations，并 MUST 在不兼容变更前保留可恢复备份；未知的新版本数据库 MUST NOT 被旧应用写入。

#### Scenario: Initialize a new profile
- **WHEN** 用户数据目录不存在数据库
- **THEN** 系统 SHALL 创建数据库并应用全部 migrations 到当前版本

#### Scenario: Migration fails
- **WHEN** migration 无法完成或完整性检查失败
- **THEN** 系统 MUST 回滚 migration、保留原数据库并进入只读诊断状态

#### Scenario: Database is newer than the application
- **WHEN** schema version 高于当前应用支持版本
- **THEN** 系统 MUST 拒绝写入并提示升级应用

### Requirement: Configuration is layered and validated
系统 SHALL 按应用默认、用户配置、项目配置、会话选择合并非敏感设置，并 MUST 在使用前按 schema 验证；项目配置 MUST NOT 覆盖硬安全边界或用户凭据来源。

#### Scenario: Merge valid project settings
- **WHEN** 项目 `.klaude/config.toml` 只覆盖允许的模型或上下文偏好
- **THEN** 系统 SHALL 在该项目会话中应用覆盖且保留用户级安全规则

#### Scenario: Project configuration is invalid
- **WHEN** 项目配置包含类型错误或未知的安全字段
- **THEN** 系统 SHALL 报告文件与字段，并 MUST 回退到最后一个完整有效配置

#### Scenario: Load project instructions
- **WHEN** 项目包含 `.klaude/instructions.md` 或受支持的 `AGENTS.md`
- **THEN** 系统 SHALL 将其作为低于 System/App 指令的项目上下文加载并记录来源

### Requirement: Sensitive settings are stored only as references
配置与数据库 SHALL 只保存凭据环境变量名或安全凭据条目 id，MUST NOT 保存解析后的 API key、token 或密码。

#### Scenario: Save provider settings
- **WHEN** 用户配置 Provider 凭据来源
- **THEN** 系统 SHALL 持久化引用并在内存中使用解析值

#### Scenario: User attempts to save a plaintext secret
- **WHEN** 设置 API 收到明文密钥字段
- **THEN** 系统 MUST 拒绝持久化并引导用户选择安全来源

### Requirement: Trace and logs are structured, bounded, and redacted
系统 SHALL 为 Agent 生命周期写入版本化 JSONL Trace，并 SHALL 使用结构化日志记录操作、时长和错误；二者 MUST 脱敏凭据与已标记敏感参数并 MUST 遵守大小/轮转限制。

#### Scenario: Trace a tool lifecycle
- **WHEN** 工具从请求进入完成状态
- **THEN** Trace SHALL 记录关联 id、状态、时间、受限结果和 rawRef，且不包含未脱敏凭据

#### Scenario: Log an internal failure
- **WHEN** 后端发生带 stack/cause 的内部错误
- **THEN** 系统 SHALL 把诊断信息写入受保护日志并只向 UI 返回脱敏错误码与消息

### Requirement: Usage records remain attributable
系统 SHALL 按 Provider、模型、session 和 turn 保存可用的 token、成本与延迟指标，未知成本 MUST 标记为 unavailable 而不是推测。

#### Scenario: Provider reports usage
- **WHEN** 模型流产生 usage 事件
- **THEN** 系统 SHALL 将指标关联到当前 Turn 并更新 UI 状态

#### Scenario: Price data is unavailable
- **WHEN** 系统没有所选模型的可信价格配置
- **THEN** 系统 SHALL 显示 token 与延迟但 MUST NOT 展示估算成本为事实
