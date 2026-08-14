# Klaude UI and MCP Redesign

## Goal

Simplify the desktop shell around conversations and MCP connections, remove unused navigation, eliminate fake model defaults, and add real MCP stdio and Streamable HTTP connections that can expose discovered tools to the agent.

## UI behavior

- The sidebar navigation contains `新对话` and `MCP`; the old `拉取请求` and `站点` pages are removed from navigation and routing. The existing agent change and workspace browsing services remain available to the core conversation flow.
- `Klaude` is a non-interactive brand label. The brand menu and its actions are removed.
- The sidebar collapse control is immediately to the left of search. The sidebar remains mounted while its width, padding, opacity, and pointer interaction transition, so the main content does not jump. The collapsed-state reopen control is below the Wails drag region.
- The `开始使用` checklist, counter, and controller state are removed. The empty conversation hero remains available as a normal chat affordance.
- Hover states change color, background, or border only. Hover transforms, hover scaling, hover elevation, and translate-Y entrance effects are removed. Functional transforms such as a switch thumb, spinner, scroll thumb, and disclosure rotation may remain where they convey state.
- The sidebar search is a single subtle bordered input with an embedded search icon and no nested thick focus frame.

## Model behavior

- Default configuration has no provider model, no default model identifier, and no preview/fake catalog.
- The frontend represents an unconfigured catalog as an empty profile list with no active ID and shows `未配置模型`.
- Creating a session or sending a message without a configured model returns an actionable configuration error.

## MCP architecture

The backend adds a small `internal/mcp` boundary over the official Go MCP SDK. A persisted server definition has an ID, name, transport (`streamable_http` or `stdio`), enabled state, URL or command/arguments, and environment-variable references. Secrets are not persisted as plaintext values.

The manager owns connection lifecycle, initialization, capability discovery, tool invocation, and shutdown. It exposes a stable application DTO to Wails and registers connected MCP tools into each per-turn tool registry with namespaced names such as `mcp__<server>__<tool>`. MCP tool results are converted to the existing tool result contract.

RPC methods cover list, save, delete, connect, and disconnect. The same React `McpManager` component is rendered by the sidebar MCP page and the settings MCP group. The page owns no transport details; it calls the controller/backend boundary and renders connection state, validation errors, and discovered tools.

Streamable HTTP uses the modern MCP transport, accepting JSON or `text/event-stream` responses and retaining negotiated session metadata. stdio launches the configured command through the SDK command transport and keeps stdout reserved for JSON-RPC messages.

## Verification

- Go tests cover config defaults/validation, MCP definition normalization, stdio/HTTP transport selection, tool name namespacing, and lifecycle cleanup.
- Frontend tests cover empty model state, navigation removal, MCP form transport fields, and sidebar collapse/search controls.
- `npm --prefix frontend run test`, `npm --prefix frontend run typecheck`, and the available Go test/build commands are run before handoff.
