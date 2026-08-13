.PHONY: dev preview test lint typecheck build package clean

dev:
	wails dev

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
	wails build

clean:
	rm -rf frontend/dist build bin
