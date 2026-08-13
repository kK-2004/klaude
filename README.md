# Klaude desktop coding agent

Klaude is a local-first Wails desktop application for inspecting and changing a
developer-selected workspace. The MVP stores projects, sessions, messages,
Turns, approvals, tool results, and reviewable file changes in a per-user
SQLite profile. Model access is OpenAI-compatible and credentials are resolved
from environment variable references; plaintext secrets are never persisted or
written to traces.

## Development

Requirements: Go 1.26+, Node.js 20+, Git, and `rg` (ripgrep). From the project
root:

```sh
npm install
go mod download
make test
make dev
```

`make build` creates a production Wails bundle. `make package` runs the same
build with the packaging target used by release CI.

Use `go test -run '^$' -bench . ./internal/context ./internal/event` to profile
context assembly and event publication. MVP limits are 120k context chars,
24k tool-result chars, 200k textual diff chars, and 24k terminal output chars;
these bounds keep streaming and review responsive on a fresh profile.

## Configuration and data

User configuration is TOML under the platform user config directory in
`Klaude/config.toml`; an optional project `.klaude/config.toml` is merged for
non-security settings. Add `AGENTS.md` or `.klaude/instructions.md` in a
workspace for deterministic project instructions. The profile contains:

- `klaude.db`: SQLite state with WAL and migrations;
- `snapshots/`: content-addressed preimages used for safe Undo;
- `traces/` and `logs/`: bounded JSONL diagnostics with redaction;
- `cache/`: disposable provider and navigation cache.

Configure a provider endpoint, model, and credential environment variable in
TOML or the Settings UI. Endpoints must use HTTPS except for loopback local
development servers.

## Safety boundary

The MVP enforces a logical workspace boundary: path resolution rejects
traversal and symlink escapes, and mutating/shell tools default to approval.
This is not an OS sandbox. Review the Changes panel before Undo or further
edits. Binary and oversized content is shown as metadata-only instead of being
rendered as executable HTML.

## Troubleshooting and privacy

Use the in-app diagnostic banner or inspect the profile logs when startup is
read-only or a capability is missing. Git and ripgrep are optional; the UI
labels unavailable capabilities and remains usable for non-Git workspaces.
Provider failures expose stable user-facing codes while stack traces stay in
protected logs. Do not paste credentials into prompts or project files.

Deferred features include OS-level sandboxing, automatic multi-agent planning,
remote credential brokers, and signed installers. See `ARCHITECTURE.md` and
the OpenSpec change artifacts for the complete MVP boundary.
