# -----------------------------
# Project Metadata
# -----------------------------
PROJECT_NAME = moh-sso-dashboard
GO_CMD = go
GO_MAIN = ./cmd/server
FRONTEND_DIR = ./frontend
BINARY_NAME = sso-server
DOCKER_IMAGE = $(PROJECT_NAME):latest
NPM ?= npm
NODE ?= node
BACKEND_VERSION ?= $(shell cat backend/VERSION)
FRONTEND_VERSION ?= $(shell $(NODE) -p "require('./frontend/apps/shell/package.json').version")
RELEASE_REMOTE ?= upstream
BUMP ?= patch
RELEASE_MAKE ?= make
.DEFAULT_GOAL := build

.PHONY: release-help release-preflight release-check release-backend-check release-frontend-check \
        release release-backend release-frontend release-prepare release-manual \
        release-backend-manual release-frontend-manual frontend-version frontend-publish
.PHONY: release-notes

release-notes:
	@$(NODE) scripts/release-notes.mjs both "$(BACKEND_VERSION)"

release-help:
	@echo 'make release                                  Automatically version, commit, verify, and release both'
	@echo 'make release-prepare                          Calculate and commit versions without publishing'
	@echo 'make release-notes                            Generate per-module and backend notes for current versions'
	@echo 'make release BUMP=minor                       Choose minor bumps (default: patch)'
	@echo 'make frontend-version                         Apply pending Changesets and refresh metadata'
	@echo 'make release-check BACKEND_VERSION=0.0.2       Verify both components locally'
	@echo 'make release-backend                          Automatically version and release the backend'
	@echo 'make release-frontend                         Automatically version and release all frontend modules'
	@echo 'make release-manual BACKEND_VERSION=0.0.2      Release previously prepared explicit versions'
	@echo 'make frontend-publish                         Verify and publish frontend npm modules'
	@echo 'FRONTEND_VERSION defaults to the shell version; RELEASE_REMOTE defaults to upstream.'

frontend-version:
	cd $(FRONTEND_DIR) && $(NPM) run version:packages

release-preflight:
	@$(NODE) scripts/release.mjs check both "$(BACKEND_VERSION)" "$(FRONTEND_VERSION)" "$(RELEASE_REMOTE)"

release-check:
	@$(MAKE) release-preflight
	@$(MAKE) release-backend-check
	@$(MAKE) release-frontend-check

release-backend-check:
	@$(NODE) scripts/release.mjs check backend "$(BACKEND_VERSION)" "$(FRONTEND_VERSION)" "$(RELEASE_REMOTE)"
	@$(MAKE) -C backend release-check VERSION="$(BACKEND_VERSION)"

release-frontend-check:
	@$(NODE) scripts/release.mjs check frontend "$(BACKEND_VERSION)" "$(FRONTEND_VERSION)" "$(RELEASE_REMOTE)"
	@$(NODE) scripts/check-frontend-release.mjs "$(NPM)" "$(NODE)"

release-prepare:
	@$(NODE) scripts/auto-release.mjs prepare both "$(BUMP)" "$(RELEASE_REMOTE)" "$(NPM)" "$(RELEASE_MAKE)"

release:
	@$(NODE) scripts/auto-release.mjs release both "$(BUMP)" "$(RELEASE_REMOTE)" "$(NPM)" "$(RELEASE_MAKE)"

release-backend:
	@$(NODE) scripts/auto-release.mjs release backend "$(BUMP)" "$(RELEASE_REMOTE)" "$(NPM)" "$(RELEASE_MAKE)"

release-frontend:
	@$(NODE) scripts/auto-release.mjs release frontend "$(BUMP)" "$(RELEASE_REMOTE)" "$(NPM)" "$(RELEASE_MAKE)"

release-manual:
	@$(MAKE) release-check
	@$(NODE) scripts/release.mjs publish both "$(BACKEND_VERSION)" "$(FRONTEND_VERSION)" "$(RELEASE_REMOTE)"

release-backend-manual:
	@$(MAKE) release-backend-check
	@$(NODE) scripts/release.mjs publish backend "$(BACKEND_VERSION)" "$(FRONTEND_VERSION)" "$(RELEASE_REMOTE)"

release-frontend-manual:
	@$(MAKE) release-frontend-check
	@$(NODE) scripts/release.mjs publish frontend "$(BACKEND_VERSION)" "$(FRONTEND_VERSION)" "$(RELEASE_REMOTE)"

frontend-publish:
	@$(MAKE) release-frontend-check
	cd $(FRONTEND_DIR) && $(NPM) run release:packages

# -----------------------------
# Go Backend Commands
# -----------------------------
.PHONY: build
build:
	@echo "🚀 Building Go backend..."
	cd backend && $(GO_CMD) build -o ../bin/$(BINARY_NAME) $(GO_MAIN)
	@echo "✅ Build complete: bin/$(BINARY_NAME)"

.PHONY: run
run:
	@echo "🏃 Running Go backend..."
	cd backend && $(GO_CMD) run $(GO_MAIN)

.PHONY: test
test:
	@echo "🧪 Running tests..."
	cd backend && $(GO_CMD) test ./... -v

.PHONY: fmt
fmt:
	@echo "🎨 Formatting code..."
	cd backend && $(GO_CMD) fmt ./...

.PHONY: tidy
tidy:
	@echo "🧹 Tidying dependencies..."
	cd backend && $(GO_CMD) mod tidy

.PHONY: clean
clean:
	@echo "🧽 Cleaning up..."
	rm -rf bin/
	@echo "✅ Clean complete"

# -----------------------------
# React Frontend Commands
# -----------------------------
.PHONY: frontend-install
frontend-install:
	@echo "📦 Installing frontend dependencies..."
	cd $(FRONTEND_DIR) && $(NPM) ci

.PHONY: frontend-build
frontend-build:
	@echo "🏗️ Building React app..."
	cd $(FRONTEND_DIR) && $(NPM) run build:docker

.PHONY: frontend-start
frontend-start:
	@echo "🌐 Starting React development server..."
	cd $(FRONTEND_DIR) && $(NPM) run dev

# -----------------------------
# Utility Commands
# -----------------------------
.PHONY: dev
dev: compose-dev-up

.PHONY: dev-native
dev-native:
	@echo "Starting native backend and frontend (infrastructure managed separately)..."
	$(MAKE) -j2 run frontend-start

.PHONY: all
all: tidy build frontend-build
	@echo "✅ All components built successfully!"


# API contracts remain application artifacts embedded by the Go server.
.PHONY: help contracts-generate contracts-check openapi-generate openapi-validate openapi-export generate frontend-manifests sqlc-generate
help: infra-help
	@echo ''
	@echo 'Contracts and code generation (run from the repo root):'
	@echo '  make contracts-generate                    Rewrite backend/docs/openapi.yaml'
	@echo '  make contracts-check                       Check API coverage/schema/freshness; no writes'
	@echo '  make openapi-export                        Export embedded spec [OPENAPI_OUTPUT=/tmp/api.yaml]'
	@echo '  make frontend-manifests                    Regenerate import-map and version JSON manifests'
	@echo '  make generate                              Regenerate API contracts and frontend manifests'
	@echo '  make sqlc-generate                         Regenerate Go SQL bindings; requires sqlc'
	@echo 'Aliases: openapi-generate = contracts-generate; openapi-validate = contracts-check.'

contracts-generate openapi-generate:
	$(MAKE) -C backend openapi-generate GO="$(GO_CMD)"

contracts-check openapi-validate:
	$(MAKE) -C backend openapi-validate GO="$(GO_CMD)"

OPENAPI_OUTPUT ?= /tmp/moh-openapi.yaml
openapi-export:
	$(MAKE) -C backend openapi-export OUTPUT="$(abspath $(OPENAPI_OUTPUT))" GO="$(GO_CMD)"

frontend-manifests:
	cd $(FRONTEND_DIR) && $(NPM) run generate:import-map && $(NPM) run generate:versions

generate: contracts-generate frontend-manifests

sqlc-generate:
	$(MAKE) -C backend sqlc-generate

INFRA_EMBEDDED := 1
include infra/Makefile
