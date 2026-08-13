## ADDED Requirements

### Requirement: Providers expose a normalized streaming contract
每个 ModelProvider SHALL 接受统一 ModelRequest，并 SHALL 只向 Runtime 暴露文本增量、工具调用生命周期、用量、完成和归一化错误事件，而不泄漏厂商 SDK 类型。

#### Scenario: Stream a text response
- **WHEN** 上游模型返回多个文本流片段
- **THEN** Provider SHALL 按接收顺序产生标准 TextDelta 并最终产生 completed 事件

#### Scenario: Upstream protocol is malformed
- **WHEN** 上游返回无法解析或违反协议的流事件
- **THEN** Provider MUST 关闭该流并返回带 provider code 的脱敏非成功错误

### Requirement: MVP supports a configurable OpenAI-compatible provider
系统 SHALL 提供一个 OpenAI 兼容 Provider，允许用户选择 endpoint 与 model，并 MUST 验证 endpoint scheme、模型标识和工具调用能力后再开始 Turn。

#### Scenario: Use the default compatible endpoint
- **WHEN** 用户选择有效模型且默认 OpenAI endpoint 已配置
- **THEN** Provider SHALL 使用统一请求中的消息、工具与输出限制发起流式请求

#### Scenario: Configure an invalid endpoint
- **WHEN** endpoint 缺少受支持的 HTTPS scheme，且未通过明确的本地开发例外
- **THEN** 系统 MUST 拒绝保存或使用该 endpoint

#### Scenario: Selected model lacks tool support
- **WHEN** 用户尝试用不支持工具调用的模型运行需要工具的 Agent 模式
- **THEN** 系统 SHALL 在请求前显示能力不匹配错误

### Requirement: Credentials are resolved without plaintext persistence
Provider 凭据 MUST 通过环境变量引用或平台安全凭据服务解析，且 MUST NOT 以明文写入 TOML、SQLite、事件、JSONL Trace 或日志。

#### Scenario: Resolve an environment credential
- **WHEN** Provider 配置引用一个存在且非空的环境变量
- **THEN** 系统 SHALL 只在内存中解析该值并用于请求授权

#### Scenario: Credential is missing
- **WHEN** 所引用的环境变量或安全凭据条目不存在
- **THEN** 系统 MUST 在网络请求前失败并显示凭据配置提示

#### Scenario: Record request diagnostics
- **WHEN** Provider 写入日志或 Trace 元数据
- **THEN** 系统 MUST 删除 Authorization header、密钥、cookie 和已标记敏感字段

### Requirement: Streaming tool calls are assembled safely
Provider SHALL 按上游 tool call id 聚合分片参数，只在收到完整调用后产生可调度事件，并 MUST 在调度前把参数作为 JSON 验证。

#### Scenario: Receive interleaved tool argument deltas
- **WHEN** 上游交错发送多个 tool call 的参数片段
- **THEN** Provider SHALL 按各自 id 独立组装并输出正确名称与完整参数

#### Scenario: Tool arguments are invalid JSON
- **WHEN** tool call 结束但聚合参数不是有效 JSON
- **THEN** Provider SHALL 产生结构化 invalid_tool_arguments 结果且 Runtime MUST NOT 执行工具

### Requirement: Provider errors and usage are normalized
Provider SHALL 把认证、限流、瞬时网络、超时、取消和协议错误映射为稳定错误代码与 retryable 属性，并 SHALL 在上游提供时报告 input、cached、output 与 reasoning token 用量。

#### Scenario: Upstream rate limit
- **WHEN** 上游返回可重试的限流响应
- **THEN** Provider SHALL 返回 retryable rate_limit 错误并保留安全的 retry-after 信息

#### Scenario: User cancellation
- **WHEN** 请求 context 被用户取消
- **THEN** Provider SHALL 立即关闭响应体并返回 cancelled 而不是 retryable 网络错误

#### Scenario: Usage fields are partially unavailable
- **WHEN** 上游只报告部分 token 指标
- **THEN** Provider SHALL 保存可用指标并将未知字段标为 unavailable，而不推测数值
