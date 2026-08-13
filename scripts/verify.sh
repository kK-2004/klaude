#!/usr/bin/env sh
set -eu

npm run typecheck
npm run lint
npm run test
npm run build
go test ./...
go vet ./...
go build ./cmd/klaude
openspec validate build-coding-agent-desktop-app --type change --strict
