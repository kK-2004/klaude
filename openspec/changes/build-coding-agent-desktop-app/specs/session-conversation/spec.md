## ADDED Requirements

### Requirement: Sessions belong to projects and are resumable
系统 SHALL 允许用户在当前项目中创建、命名、列出和重新打开会话，并 MUST 保留会话所选 Provider、模型、消息与最终状态。

#### Scenario: Create a session
- **WHEN** 用户在已打开项目中创建新会话
- **THEN** 系统 SHALL 创建一个 idle 会话并将其设为当前会话

#### Scenario: Reopen a historical session
- **WHEN** 用户选择一个已完成或失败的历史会话
- **THEN** 系统 SHALL 从持久化快照恢复有序消息、Turn、工具记录和变更摘要

### Requirement: Sending a message starts one durable Agent Turn
系统 SHALL 在接受用户消息时原子创建消息和对应 Agent Turn，返回稳定 `turnId`，并 MUST 阻止同一会话同时存在多个活动 Turn。

#### Scenario: Send a message to an idle session
- **WHEN** 用户提交非空消息且 Provider 配置有效
- **THEN** 系统 SHALL 持久化消息、创建 running Turn 并立即返回其 `turnId`

#### Scenario: Send while the session is running
- **WHEN** 用户在同一会话已有 running 或 waiting_approval Turn 时再次发送消息
- **THEN** 系统 MUST 拒绝重复启动并指示用户先等待或取消当前 Turn

#### Scenario: Submit an empty message
- **WHEN** 用户提交仅包含空白字符的消息
- **THEN** 系统 MUST 不创建消息或 Agent Turn

### Requirement: Conversation renders ordered streaming activity
系统 SHALL 按 Turn sequence 增量展示 Assistant 文本、工具状态、审批、用量、错误与完成状态，并 MUST 对重复 event id 幂等处理。

#### Scenario: Receive text deltas
- **WHEN** 当前 Turn 连续产生有序文本增量
- **THEN** 系统 SHALL 将增量追加到同一 Assistant 回复且保持原始顺序

#### Scenario: Receive a duplicate event
- **WHEN** 前端收到已处理过的 event id
- **THEN** 系统 MUST 忽略重复事件而不重复文本或工具卡片

#### Scenario: Detect an event gap
- **WHEN** 前端检测到 Turn sequence 不连续
- **THEN** 系统 SHALL 重新查询该会话的后端快照并以快照修正显示状态

### Requirement: User can cancel an active Turn
系统 SHALL 在 running 或 waiting_approval 状态提供停止操作，并 MUST 将取消传播到模型流、审批等待和正在执行的可取消工具。

#### Scenario: Cancel during model streaming
- **WHEN** 用户停止正在接收模型流的 Turn
- **THEN** 系统 SHALL 终止流、持久化 cancelled 状态并停止产生新的工具调用

#### Scenario: Cancel while awaiting approval
- **WHEN** 用户停止 waiting_approval Turn
- **THEN** 系统 SHALL 关闭待审批请求并将 Turn 标记为 cancelled

### Requirement: Terminal states are explicit and recoverable
每个 Agent Turn MUST 结束为 completed、cancelled、failed 或 interrupted 之一，系统 SHALL 向用户展示可读原因且 MUST NOT 显示未脱敏的内部堆栈或凭据。

#### Scenario: Model returns a final answer
- **WHEN** Agent Loop 收到模型完成且无待执行工具
- **THEN** 系统 SHALL 持久化 Assistant 最终消息并将 Turn 标记为 completed

#### Scenario: A non-retryable error occurs
- **WHEN** Provider、Runtime 或工具发生不可重试系统错误
- **THEN** 系统 SHALL 将 Turn 标记为 failed并显示可操作的脱敏错误

#### Scenario: Application restarts during a Turn
- **WHEN** 启动恢复发现上次退出时仍为 running 或 waiting_approval 的 Turn
- **THEN** 系统 MUST 将其标记为 interrupted 且 MUST NOT 自动重放模型请求或工具副作用
