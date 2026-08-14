# UI and MCP Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Simplify Klaude's shell, remove fake onboarding/model/navigation, and add persisted MCP stdio and Streamable HTTP connections usable by the agent.

**Architecture:** Keep React state behind the existing AppController and Wails RPC boundary. Add a backend MCP manager around the official Go SDK, persist only non-secret connection definitions in the existing TOML config, and adapt discovered MCP tools into the existing per-turn tool registry.

**Tech Stack:** Go 1.26, Wails v2, official `github.com/modelcontextprotocol/go-sdk/mcp`, React 19, TypeScript, Vitest, CSS.

**Spec:** `docs/superpowers/specs/2026-08-13-ui-mcp-redesign-design.md`

## Global Constraints

- `go.mod` keeps `module github.com/kk-2004/klaude`.
- No fake or preview model profile is shown when the catalog is empty.
- MCP secrets are represented by environment-variable references, not plaintext persisted values.
- Hover interactions change color/background/border only; sidebar collapse may animate layout properties.
- Existing user modifications, including the module-path migration and generated Wails changes, remain intact.

### Task 1: Model defaults and navigation contract

**Files:**
- Modify: `internal/config/config.go`, `internal/app/service.go`, `frontend/src/app/use-app-controller.ts`, `frontend/src/app/types.ts`, `frontend/src/App.tsx`, `frontend/src/components/Sidebar.tsx`
- Test: `internal/config/config_test.go`, `frontend/src/App.test.tsx`

- [ ] Add tests proving defaults have an empty model and the frontend catalog has no preview profile.
- [ ] Run the focused tests and observe the failure caused by the current defaults.
- [ ] Remove the fake/preview catalog and onboarding state, make model-dependent actions return a clear configuration error, and remove obsolete page navigation.
- [ ] Run the focused tests again.

### Task 2: MCP config and transport manager

**Files:**
- Modify: `go.mod`, `internal/config/config.go`, `internal/app/composition.go`, `internal/app/service.go`, `internal/app/rpc.go`, `internal/app/runtime.go`
- Create: `internal/mcp/config.go`, `internal/mcp/manager.go`, `internal/mcp/tool.go`, `internal/mcp/manager_test.go`

- [ ] Add failing tests for definition normalization, transport selection, namespaced tool definitions, and close behavior.
- [ ] Add the official SDK dependency and implement the manager with stdio and Streamable HTTP transports.
- [ ] Persist server definitions and expose list/save/delete/connect/disconnect RPC DTOs.
- [ ] Merge connected MCP tools into each per-turn registry and convert invocation results to existing tool results.
- [ ] Run focused Go tests.

### Task 3: MCP React page and settings group

**Files:**
- Create: `frontend/src/components/McpManager.tsx`, `frontend/src/pages/McpPage.tsx`
- Modify: `frontend/src/lib/backend.ts`, `frontend/src/app/types.ts`, `frontend/src/app/use-app-controller.ts`, `frontend/src/App.tsx`, `frontend/src/pages/SettingsPage.tsx`, `frontend/src/components/Sidebar.tsx`
- Test: `frontend/src/components/McpManager.test.tsx`

- [ ] Add failing tests for stdio/Streamable HTTP form switching, save/delete actions, and rendering connection state.
- [ ] Add typed backend bindings and controller methods.
- [ ] Render the shared manager in the MCP page and settings group.
- [ ] Run the focused frontend tests and typecheck.

### Task 4: Shell, search, and interaction styles

**Files:**
- Modify: `frontend/src/components/AppShell.tsx`, `frontend/src/components/Sidebar.tsx`, `frontend/src/styles.css`, `frontend/src/components/Composer.tsx`, `frontend/src/components/ModelSettings.tsx`
- Delete: `frontend/src/pages/PullRequestsPage.tsx`, `frontend/src/pages/SitesPage.tsx`, `frontend/src/pages/PluginsPage.tsx`

- [ ] Add focused DOM assertions for non-clickable branding, collapse/search controls, and removed navigation labels.
- [ ] Keep the sidebar mounted and animate its layout width; place the reopen control below the drag region.
- [ ] Remove onboarding UI, brand menu, obsolete pages, hover transforms, translate-Y scroll-thumb styles, and thick search styling.
- [ ] Run frontend tests, lint, and typecheck.

### Task 5: Full verification

**Files:**
- Modify: generated Wails bindings only if the installed Wails generator updates them.

- [ ] Run `npm --prefix frontend run test`.
- [ ] Run `npm --prefix frontend run typecheck` and `npm --prefix frontend run lint`.
- [ ] Run Go tests and `go build ./cmd/klaude` with Go 1.26+.
- [ ] Inspect `git diff --check` and report any environment-only verification blockers.
