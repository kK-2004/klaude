## ADDED Requirements

### Requirement: Application starts into a usable desktop workspace
系统 SHALL 以 Wails 桌面应用启动 React 工作区，并在后端初始化完成前显示明确的加载状态；初始化失败时 MUST 显示可操作错误而不是空白窗口。

#### Scenario: Successful startup
- **WHEN** 用户启动 Klaude 且本地数据初始化成功
- **THEN** 系统 SHALL 显示可交互的桌面工作区

#### Scenario: Startup initialization fails
- **WHEN** 配置或数据库初始化无法完成
- **THEN** 系统 MUST 显示失败原因、诊断位置和可安全重试的操作

### Requirement: User can open a local project
系统 SHALL 允许用户选择一个现有可读目录作为项目，MUST 将其规范化为唯一根路径，并 SHALL 同时支持 Git 与非 Git 目录。

#### Scenario: Open a Git project
- **WHEN** 用户选择包含 Git 工作树的可读目录
- **THEN** 系统 SHALL 创建或复用该规范路径对应的项目并显示 Git 分支与变更能力

#### Scenario: Open a non-Git project
- **WHEN** 用户选择可读但不属于 Git 工作树的目录
- **THEN** 系统 SHALL 打开项目并将 Git 能力标记为不可用，而不阻止文件与对话能力

#### Scenario: Reject an invalid project path
- **WHEN** 用户选择不存在、不是目录或不可读的路径
- **THEN** 系统 MUST 拒绝创建项目并显示具体校验错误

#### Scenario: Reopen the same canonical project
- **WHEN** 用户通过等价路径再次打开已登记的项目根目录
- **THEN** 系统 SHALL 复用原项目记录而不创建重复项目

### Requirement: Workspace provides project, conversation, and change surfaces
桌面 UI SHALL 提供项目/会话导航、对话输入与事件时间线、文件变更与 Diff 三类主要工作面，并 MUST 让当前项目、会话和运行状态始终可识别。

#### Scenario: Navigate an active project
- **WHEN** 用户选择一个已有会话的项目
- **THEN** 系统 SHALL 在同一工作区显示该项目的会话、当前对话和当前 Turn 的变更

#### Scenario: Use a narrow window
- **WHEN** 窗口宽度不足以同时容纳三栏
- **THEN** 系统 SHALL 允许折叠或切换侧栏且 MUST 保持输入、停止运行和待审批入口可发现

### Requirement: Project file navigation is scoped to the opened workspace
系统 SHALL 展示当前项目根目录内的文件和目录，并 MUST NOT 通过文件导航暴露项目根目录外的内容。

#### Scenario: Browse project files
- **WHEN** 用户展开项目中的一个目录
- **THEN** 系统 SHALL 只返回该项目规范根路径内的直接子项

#### Scenario: File entry resolves outside the project
- **WHEN** 某个符号链接或规范化路径解析到项目根目录外
- **THEN** 系统 MUST 阻止导航并标记该条目为工作区外目标

### Requirement: External capability availability is visible
系统 SHALL 在项目打开时探测 Git、ripgrep 和模型配置等 MVP 依赖，并 SHALL 在相关功能入口显示可用性与修复提示。

#### Scenario: ripgrep is unavailable
- **WHEN** 当前系统找不到受支持的 `rg` 可执行文件
- **THEN** 系统 SHALL 将 Grep/Glob 加速能力标记为不可用并提供安装或配置提示

#### Scenario: No model credential is configured
- **WHEN** 用户打开项目但当前 Provider 无可解析凭据
- **THEN** 系统 SHALL 允许浏览历史与项目，但 MUST 在发送消息前提示用户配置凭据
