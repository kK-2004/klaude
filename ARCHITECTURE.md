# Klaude package boundaries

Klaude is split into a framework-neutral Go core and a thin Wails application
adapter. Dependencies flow inward:

```text
frontend -> Wails bindings -> internal/app -> runtime ports -> infrastructure
```

- `internal/agent`, `internal/model`, `internal/context`, `internal/tool`,
  `internal/permission`, and `internal/approval` contain domain contracts and
  runtime behavior. They must not import Wails or React.
- `internal/app` is the only package that is bound to Wails. It translates RPC
  DTOs and desktop events into domain calls.
- `internal/filesystem`, `internal/search`, `internal/git`, `internal/executor`,
  `internal/storage`, `internal/config`, and `internal/trace` implement ports.
- `frontend/src` owns presentation and local UI state only. It must not contain
  filesystem, Git, shell, permission, or model-provider behavior.

The first release intentionally provides logical workspace restrictions and
approval policy, not an OS-level sandbox.
