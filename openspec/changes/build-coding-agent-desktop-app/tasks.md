## 1. Repository and Desktop Bootstrap

- [x] 1.1 Initialize the Go module, `cmd/klaude` entry point, Wails configuration, embedded frontend assets, and a minimal application lifecycle that opens a window.
- [x] 1.2 Scaffold the React + TypeScript + Vite frontend and configure Tailwind CSS, shadcn/ui conventions, Zustand, react-markdown, remark-gfm, Shiki, and Monaco Editor.
- [x] 1.3 Create the agreed backend and frontend feature directory structure and document package ownership and allowed dependency directions.
- [x] 1.4 Add reproducible development, test, lint, typecheck, production build, and Wails packaging commands with pinned toolchain requirements.
- [x] 1.5 Add CI jobs that run Go tests/static checks, frontend tests/typecheck, and at least one desktop build smoke check.
- [x] 1.6 Add architecture-boundary tests or static checks proving `internal/agent` does not import Wails and backend Tool packages do not depend on frontend/UI packages.

## 2. Domain Model and SQLite Foundation

- [x] 2.1 Define stable IDs, timestamps, status enums, domain errors, and records for Project, Session, Message, AgentTurn, ToolCall, ToolResult, Approval, FileChange, Usage, and Setting.
- [x] 2.2 Implement the platform-aware Klaude data-directory resolver and create data, database, trace, log, cache, and snapshot directories with restrictive permissions.
- [x] 2.3 Configure SQLite with foreign keys, WAL, busy timeout, integrity checks, transaction helpers, and clean shutdown behavior.
- [x] 2.4 Add embedded ordered v1 migrations for `projects`, `sessions`, `messages`, `agent_turns`, `tool_calls`, `tool_results`, `approvals`, `file_changes`, `usage`, and `settings` with indexes and relationship constraints.
- [x] 2.5 Implement Project, Session, Message, and AgentTurn repositories, including atomic user-message plus Turn creation and one-active-Turn enforcement.
- [x] 2.6 Implement ToolCall, ToolResult, Approval, FileChange, Usage, and Settings repositories with transactional lifecycle updates.
- [x] 2.7 Implement migration backup/version checks, rollback on migration failure, and read-only diagnostic mode for failed or newer schemas.
- [x] 2.8 Add repository and migration tests for fresh install, upgrade, rollback, foreign-key integrity, transaction failure, and concurrent access.

## 3. Configuration, Credentials, and Observability

- [x] 3.1 Define typed default, user, project, and session configuration schemas for UI, agent limits, models, context budgets, tools, and permissions.
- [x] 3.2 Implement deterministic layered TOML loading with whole-config validation, source-aware errors, and fallback to the last complete valid configuration.
- [x] 3.3 Load `.klaude/instructions.md` and supported `AGENTS.md` files as project-context sources below System/App instructions.
- [x] 3.4 Enforce a configuration trust policy that prevents project settings or instructions from weakening user DENY rules, workspace boundaries, credential sources, or Executor hard limits.
- [x] 3.5 Implement credential-reference types, environment-variable resolution, and a platform secure-store interface that never persists resolved secret values.
- [x] 3.6 Implement a central redactor for authorization headers, cookies, API keys, sensitive tool fields, errors, logs, events, and traces.
- [x] 3.7 Implement bounded versioned JSONL Trace writing and structured `slog` logging with rotation, restrictive permissions, and raw-result references.
- [x] 3.8 Add configuration, credential, redaction, trace-corruption, and log-rotation tests, including rejection of plaintext secrets.

## 4. Project and Session Application Services

- [x] 4.1 Implement ProjectManager path canonicalization, readability validation, canonical-path deduplication, Git-root discovery, and support for non-Git directories.
- [x] 4.2 Implement startup capability probes for Git, `rg`, Provider configuration, and platform prerequisites with actionable availability results.
- [x] 4.3 Implement project-scoped directory listing and file navigation with post-symlink workspace-boundary checks.
- [x] 4.4 Implement SessionManager create, rename, list, select, and snapshot-loading operations scoped to a Project.
- [x] 4.5 Implement the application operation that validates a message, atomically creates its Message/Turn, returns `turnId`, and starts the Runtime only after commit.
- [x] 4.6 Implement the active-Turn registry and cancel operation for running and waiting-approval Turns.
- [x] 4.7 Implement startup recovery that marks orphaned active Turns interrupted and pending Approvals expired without replaying model or tool side effects.
- [x] 4.8 Add service tests for invalid paths, canonical duplicates, symlink escapes, non-Git projects, empty messages, duplicate active Turns, cancellation, and restart recovery.

## 5. Event Bus, Context, and Agent Runtime

- [x] 5.1 Define the versioned event envelope and typed payloads for agent started, text delta, tool lifecycle, approval required, usage, error, cancelled, and finished.
- [x] 5.2 Implement an in-process EventBus/EventSink with per-Turn monotonic sequence generation and fan-out hooks for persistence, Trace, and desktop bridging.
- [x] 5.3 Define framework-neutral ModelProvider, ContextManager, ToolRegistry, Dispatcher, PermissionEngine, ApprovalManager, storage, clock, and ID-generator interfaces plus deterministic fakes.
- [x] 5.4 Implement ContextManager priority assembly, model-output reserve, conversation/tool-result budgets, deterministic truncation, raw references, and explicit context-limit failure.
- [x] 5.5 Implement the Agent Turn state machine and legal transitions across queued, running, waiting_approval, completed, cancelled, failed, and interrupted.
- [x] 5.6 Implement the Agent Loop for streaming text, aggregating completed tool calls, dispatching tools, appending correlated results, and iterating until a terminal result or max-turn limit.
- [x] 5.7 Propagate one `context.Context` through context building, Provider streams, Dispatcher, Approval waits, Tools, and Executor; discard late events after cancellation.
- [x] 5.8 Implement bounded cancellable retry/backoff for classified transient Provider failures and explicitly exclude permission denial, invalid tool arguments, and command exit codes.
- [x] 5.9 Implement read-only tool parallelism, serialized mutation/Shell execution, same-resource serialization, and one project mutation lock across sessions.
- [x] 5.10 Add Runtime tests using fakes for no-tool completion, multi-cycle tools, operational tool failures, event ordering, max turns, cancellation, retry classification, parallel reads, and mutation conflicts.

## 6. OpenAI-Compatible Model Provider

- [x] 6.1 Define normalized ModelRequest, message, tool-definition, ModelEvent, usage, and ProviderError types without external SDK types.
- [x] 6.2 Implement and validate OpenAI-compatible endpoint, model, timeout, output-budget, and declared capability configuration, including HTTPS policy and local-development exceptions.
- [x] 6.3 Implement cancellable streaming HTTP requests that resolve credentials only in memory and close response bodies on completion or cancellation.
- [x] 6.4 Implement SSE parsing for ordered text deltas, completion, partial usage, malformed frames, disconnects, and protocol errors.
- [x] 6.5 Implement interleaved tool-call assembly keyed by upstream id and JSON validation before emitting completed calls.
- [x] 6.6 Normalize authentication, rate-limit, transient network, timeout, cancellation, and protocol failures into stable codes and retryability.
- [x] 6.7 Persist and emit attributable token and latency usage while representing unavailable cached/reasoning token or cost fields explicitly.
- [x] 6.8 Add recorded-fixture Provider contract tests for text, fragmented/interleaved tools, invalid JSON, partial usage, rate limiting, disconnect, malformed data, secret redaction, and cancellation.

## 7. Tool Registry and Read-Only Coding Tools

- [x] 7.1 Implement the Tool interface, metadata, JSON Schema validation, unique-name registry, structured ToolResult, and stable validation errors.
- [x] 7.2 Implement the Dispatcher pipeline skeleton in the fixed order: validate, canonicalize, boundary-check, permission, approval, execute, normalize/limit, persist, trace, and emit.
- [x] 7.3 Implement a reusable workspace resolver that handles relative/absolute inputs, non-existent write targets, existing symlinks, canonical roots, and platform path semantics.
- [x] 7.4 Implement bounded ToolResult normalization that preserves useful head/tail and error summaries, sets `truncated`, and writes redacted full output behind `rawRef`.
- [x] 7.5 Implement `read_file` with line ranges, line numbers, encoding handling, binary rejection, cancellation, and output limits.
- [x] 7.6 Implement `list_directory` with deterministic ordering, file metadata, pagination/limits, and out-of-workspace symlink markers.
- [x] 7.7 Implement `glob` and `grep` through explicit `rg` argv with project scope, ignore behavior, match metadata, result limits, cancellation, and dependency-unavailable errors.
- [x] 7.8 Register the read-only tools with correct safety/concurrency metadata and expose their schemas to ModelRequest construction.
- [x] 7.9 Add tool tests for unknown names, invalid schemas, path traversal, symlink escape, line ranges, binary files, large output, `rg` absence, cancellation, and concurrent reads.

## 8. Permission, Approval, Shell, and Git

- [x] 8.1 Implement PermissionEngine rule parsing, precedence, matched-rule auditing, safe defaults, fail-closed unknown actions, and immutable hard DENY boundaries.
- [x] 8.2 Implement ApprovalManager creation and waiting with risk description, normalized argument summary, working directory, sensitive-field redaction, and request-summary hash.
- [x] 8.3 Implement idempotent `allow_once`, `reject`, cancel, and expire resolutions bound to approval id, active Turn, toolCall id, pending status, and unchanged request hash.
- [x] 8.4 Implement LocalExecutor with explicit executable/argv, canonical working directory, environment allowlist, timeout, bounded stdout/stderr, exit metadata, and context cancellation.
- [x] 8.5 Implement and test platform-specific process-tree termination so cancelled or timed-out commands do not leave managed children running.
- [x] 8.6 Implement the non-interactive `shell` tool with exact approval presentation and structured success, non-zero exit, timeout, and cancellation results.
- [x] 8.7 Implement GitService and `git_status`/`git_diff` tools using explicit system Git argv inside the discovered Git root without repository mutation.
- [x] 8.8 Add Permission/Approval tests for ALLOW/ASK/DENY, workspace escape, project privilege escalation, hash mismatch, duplicate resolution, rejection, pending cancellation, expiry, and secret redaction.
- [x] 8.9 Add Executor/Shell/Git tests for working-directory confinement, environment filtering, output limits, non-zero exit, timeout, process-tree cancellation, non-Git projects, and missing Git.

## 9. File Mutation, Changesets, Diff, and Undo

- [x] 9.1 Implement a content-addressed snapshot store that records existing/absent state, content hash, metadata, and restrictive storage permissions before any mutation.
- [x] 9.2 Implement atomic file replacement utilities that preserve the original on failure and verify an expected baseline hash immediately before commit.
- [x] 9.3 Implement `write_file` with workspace validation, approval, snapshot-before-write, no-change detection, encoding policy, and FileChange creation.
- [x] 9.4 Implement exact-context `apply_patch` parsing and validation that rejects stale or ambiguous baselines without fuzzy modification.
- [x] 9.5 Generate and persist unified diffs, before/after hashes, added/deleted line counts, file status, and Project/Session/Turn/ToolCall associations before emitting tool completion.
- [x] 9.6 Implement Turn changeset aggregation and queries that retain successful earlier changes even when a later tool or the Turn fails.
- [x] 9.7 Implement current-content divergence checks and safe retrieval of recorded before, recorded after, and current content for Diff presentation.
- [x] 9.8 Implement `UndoTurn` preflight across the complete reverse changeset, conflict reporting before any write, atomic restoration, safe removal of matching newly-created files, and idempotent undo records.
- [x] 9.9 Add mutation tests for snapshot failure, matching/stale patches, partial write failure, no-change writes, multiple files, failed Turns with changes, binary/large diffs, undo success, undo conflict, and repeated undo.

## 10. Wails Composition and Backend API

- [x] 10.1 Build the startup composition root in dependency order: config, database/migrations, recovery, EventBus, repositories, Executor, permission/approval, services/tools, Provider, ContextManager, Runtime, managers, and AppService.
- [x] 10.2 Implement AppService RPC DTOs and validation for project open/list, directory browse, capability status, session create/rename/list/load, and settings/model queries.
- [x] 10.3 Implement AppService RPCs for send message, cancel Turn, resolve Approval, load conversation snapshots, query changes/Diff, and undo Turn.
- [x] 10.4 Implement EventBridge conversion from internal envelopes to a single versioned Wails event channel without exposing domain internals or secrets.
- [x] 10.5 Generate or maintain typed TypeScript bindings for RPC DTOs, events, enums, errors, and resolution commands, and add contract compatibility tests.
- [x] 10.6 Map internal errors to stable user-facing codes/messages with diagnostic references while keeping stack traces and causes in protected logs.
- [x] 10.7 Add AppService integration tests that use a temporary database/workspace and fake Provider to verify transaction-before-start, snapshots, event order, approvals, cancellation, and undo.

## 11. Frontend Workspace and State Synchronization

- [x] 11.1 Implement the application loading, ready, read-only diagnostic, and fatal initialization states with retry/diagnostic actions.
- [x] 11.2 Implement typed backend clients and Zustand stores for app, project, session, chat, agent, approval, changes, and settings, with backend snapshots as the source of truth.
- [x] 11.3 Implement the responsive three-surface layout for project/session/file navigation, conversation/composer, and changes/Diff with persistent panel sizing and narrow-window switching.
- [x] 11.4 Implement project open/reopen UI, canonical duplicate handling, recent project navigation, current root/branch display, and non-Git/missing-capability badges with remediation.
- [x] 11.5 Implement project-scoped lazy file navigation that marks blocked external symlinks and never requests arbitrary paths outside the backend-provided tree.
- [x] 11.6 Implement session create, rename, select, historical status, model selection, and snapshot restoration views.
- [x] 11.7 Implement the event reducer with per-Turn sequence ordering, event-id deduplication, gap detection, and automatic snapshot reconciliation.
- [x] 11.8 Add frontend tests for initialization failures, responsive navigation, project capabilities, session switching, duplicate events, event gaps, and snapshot reconciliation.

## 12. Conversation, Tool Activity, and Approval UI

- [x] 12.1 Implement the conversation timeline for user/Assistant messages, streaming text, reasoning/status placeholders, tool calls/results, usage, cancellation, failure, and interrupted Turns.
- [x] 12.2 Implement sanitized Markdown rendering with raw HTML disabled, safe links, Shiki code blocks, copy actions, and bounded rendering for large tool output.
- [x] 12.3 Implement the composer with empty-input prevention, send/keyboard behavior, active-Turn locking, missing-credential guidance, and an always-visible Stop action while running or awaiting approval.
- [x] 12.4 Implement tool activity cards that display pending/running/completed/failed state, safe argument summaries, structured results, truncation, duration, and raw diagnostic references.
- [x] 12.5 Implement Approval UI that prominently displays exact command or file target, canonical working directory, timeout, risk, and reason without exposing redacted fields.
- [x] 12.6 Wire `allow_once` and `reject` with pending-state locking, idempotent responses, stale/hash-mismatch handling, and automatic closure on cancel/expiry.
- [x] 12.7 Implement status-bar model, context usage, token/cost availability, project path, Git branch, and Turn status indicators without fabricating unavailable cost.
- [x] 12.8 Add frontend tests for text streaming, terminal state display, unsafe Markdown, Stop behavior, approval detail/resolution, stale approvals, tool truncation, and unavailable usage fields.

## 13. Change Review, Undo, and Settings UI

- [x] 13.1 Implement the Changes panel grouped by Agent Turn with file status, added/deleted line totals, failed-Turn visibility, and incremental updates.
- [x] 13.2 Integrate Monaco Diff Editor for recorded before/after text and display a clear marker plus current-content option when the workspace has diverged.
- [x] 13.3 Implement safe metadata-only fallbacks for binary or oversized diffs and verify project content cannot execute HTML/script or load untrusted remote resources.
- [x] 13.4 Implement the Undo Turn flow with preflight confirmation, all-or-nothing conflict display, successful restoration refresh, and already-undone handling.
- [x] 13.5 Implement Settings UI for Provider endpoint/model/capability, credential references, context/turn limits, theme, and non-sensitive user permissions with schema validation.
- [x] 13.6 Label the MVP execution boundary as logical workspace restriction rather than an OS sandbox and surface safe default permission behavior in Settings/help.
- [x] 13.7 Add UI tests for multi-file Turn grouping, recorded/current divergence, binary/large fallback, unsafe diff content, undo success/conflict, plaintext-secret rejection, and invalid settings.

## 14. End-to-End Verification and Release Readiness

- [x] 14.1 Build a deterministic fake-Provider end-to-end fixture that emits text, read tools, an approved patch, usage, and a final answer.
- [x] 14.2 Add Wails end-to-end coverage for startup, opening Git and non-Git projects, creating a session, streaming a Turn, approving a patch, viewing Diff, and undoing the Turn.
- [x] 14.3 Add end-to-end cancellation coverage during model streaming, approval wait, `rg`, and Shell, verifying no late tool execution or orphaned managed process.
- [x] 14.4 Add restart-recovery coverage proving interrupted Turns and expired Approvals are visible, recorded file changes remain reviewable, and no side effect is replayed.
- [x] 14.5 Run security regression tests for traversal, symlink escape, stale approval/hash substitution, project config escalation, secret leakage, unsafe Markdown/Diff, and stale-baseline writes.
- [x] 14.6 Add opt-in real OpenAI-compatible smoke tests that require environment credential references and never run with or record secrets in default CI.
- [x] 14.7 Verify production builds on the declared supported OS matrix, including platform data paths, process cancellation, Git/`rg` probes, WebView behavior, signing prerequisites, and fresh-profile migration.
- [x] 14.8 Profile long streaming responses, large repository navigation, bounded tool output, Diff limits, event batching, and SQLite contention; document and enforce acceptable MVP thresholds.
- [x] 14.9 Write user/developer documentation for setup, secure credential configuration, supported capabilities, data locations, troubleshooting, privacy/redaction, logical isolation limits, and explicitly deferred features.
- [x] 14.10 Run `openspec validate --change build-coding-agent-desktop-app --strict`, all automated checks, a clean production package build, and the complete MVP acceptance checklist before marking the change implemented.
