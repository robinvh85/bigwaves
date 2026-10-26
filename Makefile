SHELL := /bin/bash
BIN := backend/bin/bigwaves
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

.PHONY: help gen gen-go gen-sqlc gen-ts dev dev-be dev-fe test test-be test-fe lint lint-be lint-fe build build-fe build-be release clean

help: ## Show targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

gen: gen-go gen-sqlc gen-ts ## Regenerate code from api/openapi.yaml and SQL

gen-go:
	cd backend && go tool oapi-codegen -config internal/httpapi/oapi-codegen.yaml ../api/openapi.yaml

gen-sqlc:
	cd backend && sqlc generate

gen-ts:
	cd frontend && npm run gen:api

# Dev ports: 8080 is often taken by a local nginx, so the backend listens on 18080 in dev.
DEV_BACKEND ?= 127.0.0.1:18080

dev: ## Run backend (:18080) and frontend (:5173) together
	@trap 'kill 0' EXIT; $(MAKE) dev-be & $(MAKE) dev-fe & wait

# Loads ../.env (repo root) when present.
dev-be:
	cd backend && set -a && { [ ! -f ../.env ] || . ../.env; } && set +a && HTTP_ADDR=$(DEV_BACKEND) go run ./cmd/bigwaves

dev-fe:
	cd frontend && BACKEND_URL=http://$(DEV_BACKEND) npm run dev

test: test-be test-fe ## Run all tests

test-be:
	cd backend && go test -race ./...

test-fe:
	cd frontend && npm run typecheck && npm test -- --run

lint: lint-be lint-fe ## Run all linters

lint-be:
	cd backend && golangci-lint run ./...

lint-fe:
	cd frontend && npm run lint

build-fe:
	cd frontend && npm run build
	rm -rf backend/internal/web/dist && mkdir -p backend/internal/web/dist
	cp -R frontend/dist/. backend/internal/web/dist/
	touch backend/internal/web/dist/.gitkeep

build-be:
	cd backend && CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -ldflags="-s -w" -o bin/bigwaves ./cmd/bigwaves
	cd backend && CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -ldflags="-s -w" -o bin/bwctl ./cmd/bwctl

build: build-fe build-be ## Build one binary with the frontend embedded

release: ## Build linux/amd64 release binaries
	$(MAKE) build GOOS=linux GOARCH=amd64

clean:
	rm -rf backend/bin frontend/dist
	find backend/internal/web/dist -mindepth 1 ! -name .gitkeep -delete
