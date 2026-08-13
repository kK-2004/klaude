## ADDED Requirements

### Requirement: Agent Runtime is independent of the desktop framework
Agent Runtime SHALL 仅通过 ModelProvider、ContextManager、ToolRegistry、Permission/Approval、EventSink 与存储接口工作，并 MUST NOT 导入 Wails 或 React 类型。

#### Scenario: Run with test adapters
- **WHEN** 测试注入 fake Provider、临时存储和内存 EventSink
- **THEN** Agent Runtime SHALL 在不启动 Wails 或 WebView 的情况下完成一个 Turn

### Requirement: Agent Loop executes model and tool cycles until a terminal result
Agent Loop SHALL 构建上下文、消费模型流、执行已完成的工具调用、追加结构化工具结果并继续请求模型，直到最终回答或终止条件发生。

#### Scenario: Complete without a tool call
- **WHEN** Provider 只产生文本并发送 completed 事件
- **THEN** Runtime SHALL 保存最终 Assistant 内容并完成 Turn

#### Scenario: Complete after a tool call
- **WHEN** Provider 请求工具且工具成功返回结果
- **THEN** Runtime SHALL 把结果关联到原 tool call、加入下一轮上下文并继续运行

#### Scenario: Tool returns an operational failure
- **WHEN** Shell 以非零 exit code 结束或工具返回可预期失败
- **THEN** Runtime SHALL 将其作为结构化 tool result 提供给模型而不是使进程崩溃

#### Scenario: Maximum turn limit is reached
- **WHEN** 模型循环达到配置的最大轮次数仍未结束
- **THEN** Runtime MUST 停止继续请求并以明确的 limit error 结束 Turn

### Requirement: Runtime emits versioned ordered events
Runtime SHALL 为每个 Turn 发出带 version、eventId、sequence、occurredAt、projectId、sessionId、turnId、type 和 payload 的事件，且 sequence MUST 在该 Turn 内单调递增。

#### Scenario: Observe a normal lifecycle
- **WHEN** 一个 Turn 经历启动、文本、工具和完成
- **THEN** EventSink SHALL 按递增 sequence 收到 started、delta、tool lifecycle 和 finished 事件

#### Scenario: Observe an error lifecycle
- **WHEN** Turn 因错误终止
- **THEN** Runtime SHALL 在终态前发出一个脱敏 error 事件且 MUST NOT 再发出 finished 成功事件

### Requirement: Cancellation propagates through the complete execution chain
Runtime MUST 为 Provider、ContextManager、Dispatcher、Approval wait、Tool 和 Executor 传递同一个可取消上下文，并 SHALL 在收到取消后停止调度新工作。

#### Scenario: Cancel a long-running tool
- **WHEN** Turn context 在 Shell 或搜索工具运行期间被取消
- **THEN** Dispatcher SHALL 请求终止子进程并使 Turn 在有界时间内进入 cancelled

#### Scenario: Late provider event arrives after cancellation
- **WHEN** Provider 在取消后仍产生一个迟到事件
- **THEN** Runtime MUST 丢弃该事件且 MUST NOT 启动其中的工具调用

### Requirement: Tool concurrency follows metadata and project mutation locks
Runtime SHALL 只并行执行声明为 read-only 且 concurrent 的独立工具，并 MUST 串行执行写工具、Shell 和同资源调用；同一项目同时 MUST 至多有一个持有变更锁的 Turn。

#### Scenario: Parallel independent reads
- **WHEN** 模型在同一批次请求多个不同文件的只读工具
- **THEN** Runtime MAY 并行执行这些调用并按 tool call id 关联各自结果

#### Scenario: Multiple mutations target one project
- **WHEN** 两个会话尝试对同一项目执行有副作用工具
- **THEN** Runtime MUST 阻止第二个 Turn 获得变更锁直到第一个释放或取消

### Requirement: Context building is deterministic and budgeted
ContextManager SHALL 按 System/App、项目指令、会话设置、对话、工具结果与显式代码上下文的稳定优先级构建请求，MUST 为模型输出预留预算，并 MUST 限制过大的工具结果。

#### Scenario: Tool output exceeds its budget
- **WHEN** 一个工具结果大于配置的上下文份额
- **THEN** ContextManager SHALL 使用带 truncated 标志与 raw reference 的受限表示而不是注入完整输出

#### Scenario: Required context cannot fit
- **WHEN** 不能在保留必需指令和输出预算的同时构建有效请求
- **THEN** Runtime MUST 以可操作的 context limit error 停止且 MUST NOT 静默删除必需指令

### Requirement: Retry behavior follows error classification
Runtime SHALL 仅重试标记为 retryable 的 Provider 限流或瞬时网络错误，并 MUST 使用有上限的退避和可取消等待；权限拒绝、无效工具参数和 Shell 非零退出 MUST NOT 触发系统级自动重试。

#### Scenario: Retry a transient provider failure
- **WHEN** Provider 返回 retryable 限流错误且重试预算仍可用
- **THEN** Runtime SHALL 等待退避时间后重试并记录尝试次数

#### Scenario: Do not retry permission denial
- **WHEN** PermissionEngine 拒绝工具调用
- **THEN** Runtime MUST 不自动重新执行该工具并 SHALL 把拒绝结果返回给模型
