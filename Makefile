.PHONY: dev preview test lint typecheck build package clean

WAILS ?= $(shell go env GOPATH)/bin/wails
GO_BUILD_ENV = GOCACHE=$(CURDIR)/.cache/go-build GOPATH=$(CURDIR)/.cache/go-path GOMODCACHE=$(CURDIR)/.cache/go-mod

dev:
	$(WAILS) dev

preview:
	npm --prefix frontend run dev

test:
	go test ./...
	npm --prefix frontend run test

lint:
	go vet ./...
	npm --prefix frontend run lint

typecheck:
	npm --prefix frontend run typecheck

build:
	npm --prefix frontend run build
	go build ./cmd/klaude

package:
	$(GO_BUILD_ENV) $(WAILS) build

clean:
	rm -rf frontend/dist build bin
