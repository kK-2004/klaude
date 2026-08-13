## ADDED Requirements

### Requirement: Every tool call receives a backend permission decision
Dispatcher MUST 在任何系统副作用前把规范化工具请求交给 PermissionEngine，并 SHALL 只根据 `ALLOW`、`ASK` 或 `DENY` 决策继续。

#### Scenario: Permission allows the request
- **WHEN** 规范化请求匹配 ALLOW 策略
- **THEN** Dispatcher SHALL 直接执行并记录命中的策略规则

#### Scenario: Permission denies the request
- **WHEN** 规范化请求匹配 DENY 策略
- **THEN** Dispatcher MUST 不创建可绕过的审批且 MUST 不执行系统操作

#### Scenario: Permission asks the user
- **WHEN** 规范化请求匹配 ASK 策略
- **THEN** Dispatcher SHALL 暂停该调用并创建待处理 Approval

### Requirement: Safe MVP defaults are enforced
默认策略 SHALL 允许项目内只读文件/Git 操作，SHALL 询问文件写入与 Shell，并 MUST 拒绝工作区逃逸和明确禁止的命令；未识别的有副作用工具 MUST 默认为 ASK 或 DENY，不能默认为 ALLOW。

#### Scenario: Read a project file with defaults
- **WHEN** `read_file` 目标位于项目内且无更严格规则
- **THEN** PermissionEngine SHALL 返回 ALLOW

#### Scenario: Write a project file with defaults
- **WHEN** `apply_patch` 目标位于项目内且无明确 allow/deny 规则
- **THEN** PermissionEngine SHALL 返回 ASK

#### Scenario: Target outside the workspace
- **WHEN** 任意工具的规范目标位于项目根目录外
- **THEN** PermissionEngine MUST 返回 DENY

### Requirement: Approval requests disclose exact execution risk
每个 Approval SHALL 包含唯一 id、session/turn/toolCall 关联、工具名称、规范化参数摘要、工作目录、风险等级、人类可读原因和请求摘要 hash，且敏感值 MUST 被脱敏。

#### Scenario: Present a Shell approval
- **WHEN** Shell 调用的权限决策为 ASK
- **THEN** UI SHALL 在允许执行前显示完整命令语义、工作目录、超时与风险说明

#### Scenario: Present a file-write approval
- **WHEN** 写文件或补丁调用的权限决策为 ASK
- **THEN** UI SHALL 显示目标项目相对路径与拟议变更摘要

### Requirement: An approval resolution is bound to one unchanged call
系统 SHALL 支持 `allow_once` 与 `reject`，并 MUST 验证 approval id、pending 状态、调用摘要 hash 和活动 Turn；过期、重复或被篡改的 resolution MUST 被拒绝。

#### Scenario: Allow once
- **WHEN** 用户对仍匹配原请求的 pending Approval 选择 allow_once
- **THEN** Dispatcher SHALL 只执行该次 tool call 一次并把 Approval 标记为 approved

#### Scenario: Reject a request
- **WHEN** 用户选择 reject
- **THEN** 系统 MUST 不执行工具，并 SHALL 将结构化 permission_rejected 结果返回 Agent

#### Scenario: Approval payload changed
- **WHEN** resolution 对应的工具参数或请求摘要与 pending Approval 不同
- **THEN** 系统 MUST 拒绝 resolution 并保持系统操作未执行

#### Scenario: Resolve the same approval twice
- **WHEN** 已终结 Approval 再次收到 resolution
- **THEN** 系统 MUST 幂等拒绝第二次执行且 MUST NOT 重复调用工具

### Requirement: Waiting approvals can be cancelled and recovered safely
Turn 等待审批时 SHALL 保持可取消；应用退出或 Turn 终止后，关联 Approval MUST 失效且 MUST NOT 在重启后自动执行。

#### Scenario: Cancel pending approval
- **WHEN** 用户取消 waiting_approval Turn
- **THEN** 系统 SHALL 将 Approval 标记为 cancelled 并唤醒等待中的 Dispatcher 返回取消结果

#### Scenario: Restart with a pending approval
- **WHEN** 应用启动恢复发现未终结 Approval
- **THEN** 系统 MUST 将其标记为 expired 且 MUST NOT 恢复执行原工具

### Requirement: Lower-trust configuration cannot weaken hard safety boundaries
项目级配置和项目指令 MUST NOT 覆盖用户级 DENY、工作区边界、凭据保护或 Executor 硬限制；无效或未知权限规则 MUST 采用更严格结果。

#### Scenario: Project attempts to allow workspace escape
- **WHEN** 项目配置声明允许读取或写入项目根目录外路径
- **THEN** 系统 MUST 忽略该放宽规则、报告配置错误并继续拒绝越界

#### Scenario: Permission rule cannot be parsed
- **WHEN** 配置包含格式错误或未知 action
- **THEN** PermissionEngine MUST 不把该规则解释为 ALLOW并 SHALL 呈现可定位错误
