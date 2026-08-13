## ADDED Requirements

### Requirement: Every file mutation creates a write-before snapshot
`write_file` 与 `apply_patch` MUST 在修改前保存目标的存在状态、内容或内容引用、hash 与元数据，并 MUST 在快照持久化成功后才写入工作区。

#### Scenario: Modify an existing file
- **WHEN** 已批准的工具准备修改现有文件
- **THEN** 系统 SHALL 在写入前保存可验证的 before snapshot

#### Scenario: Create a new file
- **WHEN** 已批准的工具准备创建不存在的文件
- **THEN** 系统 SHALL 记录 before state 为 absent 后再创建文件

#### Scenario: Snapshot cannot be persisted
- **WHEN** 系统无法可靠保存或关联 before snapshot
- **THEN** 工具 MUST 拒绝文件修改

### Requirement: Successful mutations produce complete FileChange records
每次成功文件修改 SHALL 产生与 project、session、turn 和 toolCall 关联的 FileChange，包含项目相对路径、状态、before/after hash、统一 diff 及增加/删除行统计。

#### Scenario: Patch changes a file
- **WHEN** `apply_patch` 成功提交内容
- **THEN** 系统 SHALL 在工具返回前持久化对应 FileChange

#### Scenario: A tool performs no content change
- **WHEN** 写入后的内容 hash 等于写入前 hash
- **THEN** 系统 SHALL 返回 no_change 且 MUST NOT 创建误导性的 modified FileChange

### Requirement: Changes are grouped by Agent Turn
系统 SHALL 将一个用户请求产生的全部 FileChange 聚合为该 Agent Turn 的 changeset，并 SHALL 在运行中增量更新、在终态后保持可查询。

#### Scenario: One Turn changes multiple files
- **WHEN** Agent 在同一 Turn 成功修改多个文件
- **THEN** 变更面板 SHALL 在一个 Turn 分组下列出所有文件及总行数统计

#### Scenario: Turn fails after an earlier successful change
- **WHEN** Turn 在至少一次文件写入成功后因后续错误失败
- **THEN** 系统 SHALL 保留并突出显示已发生的变更，而不是因 Turn 失败隐藏它们

### Requirement: User can inspect before and after content in a Diff viewer
变更面板 SHALL 显示新增、修改文件及行数统计，并 SHALL 使用 Monaco Diff Editor 展示持久化 before 与当前/记录 after 内容；二进制或过大文件 MUST 使用安全降级视图。

#### Scenario: Open a text diff
- **WHEN** 用户选择一个普通文本 FileChange
- **THEN** 系统 SHALL 并排或内联显示 before/after、路径和变更统计

#### Scenario: Current file diverged after the Turn
- **WHEN** 用户查看的文件当前 hash 已不同于记录的 after hash
- **THEN** 系统 SHALL 明确标记工作区已继续变化并区分 recorded after 与 current content

#### Scenario: Diff is not safely renderable
- **WHEN** 文件为二进制或超过 Diff 渲染上限
- **THEN** 系统 SHALL 显示元数据与原因且 MUST NOT 把任意字节注入文本渲染器

### Requirement: User can undo a Turn without overwriting later edits
系统 SHALL 支持按 Agent Turn 撤销其 FileChange 的逆序集合；每个目标只有在当前 hash 等于记录的 after hash 时才能自动恢复，整个撤销 MUST 在预检任何冲突后才开始。

#### Scenario: Undo an unchanged modified file
- **WHEN** 当前内容仍等于该 Turn 的 after hash 且 before snapshot 可用
- **THEN** 系统 SHALL 原子恢复 before 内容并记录 undo 结果

#### Scenario: Undo a newly created file
- **WHEN** 当前新文件仍等于记录的 after hash
- **THEN** 系统 SHALL 删除该文件并记录恢复为 absent

#### Scenario: Later edits conflict with undo
- **WHEN** 任一目标当前 hash 与该 Turn 的 after hash 不同
- **THEN** 系统 MUST 在写入任何目标前拒绝整次自动撤销并列出冲突文件

#### Scenario: Undo already reverted Turn
- **WHEN** 用户再次请求撤销一个已成功撤销的 Turn
- **THEN** 系统 MUST 不重复修改文件并 SHALL 返回 already_undone

### Requirement: Diff content is treated as untrusted data
UI MUST 将文件内容、模型文本和工具输出视为不可信内容，MUST 禁用 Markdown 原始 HTML 执行，并 MUST NOT 因查看 Diff 执行项目代码或加载不受信任的远程资源。

#### Scenario: Diff contains HTML or script text
- **WHEN** 文件内容包含可执行 HTML、脚本标签或事件处理属性
- **THEN** 系统 SHALL 以纯代码内容显示且 MUST NOT 执行它们
