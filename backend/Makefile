# ============================================================
#                 📦 Project Variables
# ============================================================
APP_NAME = moh-sso-dashboard
CLI_NAME = moh-sso
BIN_DIR = bin
MAIN_PACKAGE = ./cmd/server
CLI_PACKAGE = ./cmd/cli
GO_FILES = $(shell find . -type f -name '*.go' -not -path "./vendor/*")

VERSION ?= $(shell sh ./scripts/build-version.sh)
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || printf 'none')
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
DIRTY ?= $(shell if [ -n "$$(git status --porcelain 2>/dev/null)" ]; then printf 'true'; else printf 'false'; fi)
VERSION_PACKAGE = github.com/moh-sso-dashboard/internal/version
REPO ?= mohuganda/moh-sso-dashboard
BASE ?= main
BUMP ?= patch
BACKEND_IMAGE ?= ghcr.io/mohuganda/moh-sso-dashboard-backend
RELEASE_REMOTE ?= upstream

LDFLAGS := -s -w \
	-X '$(VERSION_PACKAGE).Version=$(VERSION)' \
	-X '$(VERSION_PACKAGE).Commit=$(COMMIT)' \
	-X '$(VERSION_PACKAGE).BuildTime=$(BUILD_TIME)' \
	-X '$(VERSION_PACKAGE).Dirty=$(DIRTY)'

# ============================================================
#                 🔧 Configuration
# ============================================================
DB_NAME               ?= sso_dashboard
DB_URL                ?= postgresql://postgres:postgres@127.0.0.1:5436/$(DB_NAME)?sslmode=disable
MIGRATE_PATH          := internal/db/migrations
COMPOSE_ENV           ?= dev

# ============================================================
#                 🔨 Go Commands
# ============================================================
GO       = go
GOTEST   = $(GO) test
GOTIDY   = $(GO) mod tidy
GOINSTALL= $(GO) install
GOBUILD  = $(GO) build
GORUN    = $(GO) run
GOFMT    = gofmt
GOLINT   = golangci-lint run

.DEFAULT_GOAL := help

# ============================================================
#        🛠️ General Utility Commands
# ============================================================

help: ## Show available commands
	@echo ""
	@echo "📘 Available Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""

tidy: ## Clean and tidy up go modules
	$(GOTIDY)

fmt: ## Format Go files
	@echo "Formatting code..."
	@$(GOFMT) -s -w $(GO_FILES)

lint: ## Run static analysis
	@echo "Running linter..."
	@$(GOLINT)

test: ## Run all tests
	@echo "Running tests..."
	@$(GOTEST) ./... -v

.PHONY: openapi-generate openapi-validate openapi-export contracts-generate contracts-check sqlc-generate
openapi-generate: ## Generate Swagger/OpenAPI documentation for every registered endpoint
	$(GORUN) ./cmd/cli openapi generate

openapi-validate: ## Verify OpenAPI contracts match routes and handler types
	$(GORUN) ./cmd/cli openapi validate

OUTPUT ?= /tmp/moh-openapi.yaml
openapi-export: ## Export the embedded OpenAPI contract to OUTPUT
	$(GORUN) ./cmd/cli openapi export --output "$(OUTPUT)"

contracts-generate: openapi-generate ## Regenerate API contracts
contracts-check: openapi-validate ## Check contract coverage and freshness

sqlc-generate: ## Regenerate Go database query contracts (requires sqlc)
	sqlc generate

architecture: ## Run backend architecture boundary checks
	@echo "Checking backend architecture boundaries..."
	@$(GOTEST) ./internal/architecture -v

deps: ## Install go dependencies
	@echo "📦 Installing dependencies..."
	$(GO) mod download

verify: fmt lint architecture test ## Run code checks before commit
	@echo "✅ All checks passed!"

# ============================================================
#        🚀 Build & Run
# ============================================================

build: ## Build the Go binary
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(APP_NAME) $(MAIN_PACKAGE)
	@echo "✅ Build complete: $(BIN_DIR)/$(APP_NAME)"

build-release: ## Build a clean semantic backend release binary
	@printf '%s' "$(VERSION)" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$$' || { echo "VERSION must be semantic"; exit 1; }
	@if [ "$(DIRTY)" != "false" ]; then echo "release builds require DIRTY=false"; exit 1; fi
	@if [ -n "$$(git status --porcelain)" ]; then echo "release builds require a clean Git worktree"; exit 1; fi
	@$(MAKE) build VERSION="$(VERSION)" COMMIT="$(COMMIT)" BUILD_TIME="$(BUILD_TIME)" DIRTY="$(DIRTY)"
	@./scripts/verify-version.sh "$(BIN_DIR)/$(APP_NAME)" "$(VERSION)" "$(COMMIT)" "$(DIRTY)"

build-cli: ## Build the management CLI with the same build metadata
	@echo "Building $(CLI_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(CLI_NAME) $(CLI_PACKAGE)
	@echo "✅ Build complete: $(BIN_DIR)/$(CLI_NAME)"

run: ## Run the Go application
	@echo "🚀 Starting $(APP_NAME)..."
	$(GORUN) $(MAIN_PACKAGE)

clean: ## Remove binaries and temp files
	@echo "🧹 Cleaning up..."
	@rm -rf $(BIN_DIR)


version: ## Show resolved backend build metadata
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Dirty: $(DIRTY)"

verify-version: ## Build and verify deterministic linker metadata
	@$(MAKE) build VERSION=1.2.3 COMMIT=abc123 BUILD_TIME=2026-06-19T12:00:00Z DIRTY=false
	@./scripts/verify-version.sh "$(BIN_DIR)/$(APP_NAME)" "1.2.3" "abc123" "false"

next-version: ## Print the next backend version: make next-version BUMP=patch|minor|major
	@./scripts/next-backend-version.sh "$(BUMP)"

release-check: ## Run local backend release verification: make release-check VERSION=1.2.3
	@case "$(VERSION)" in dev|0.0.0-dev.*) echo "VERSION is required, for example make release-check VERSION=1.2.3"; exit 2 ;; esac
	@./scripts/release-check.sh "$(VERSION)"

release-pr: ## Create a backend release PR: make release-pr VERSION=1.2.3 [REPO=owner/repo] [BASE=main] [RELEASE_REMOTE=upstream]
	@case "$(VERSION)" in dev|0.0.0-dev.*) echo "VERSION is required, for example make release-pr VERSION=1.2.3"; exit 2 ;; esac
	@./scripts/create-release-pr.sh "$(VERSION)" "$(REPO)" "$(BASE)" "$(RELEASE_REMOTE)"

release-dispatch: ## Dispatch backend release workflow: make release-dispatch VERSION=1.2.3 [REPO=owner/repo]
	@case "$(VERSION)" in dev|0.0.0-dev.*) echo "VERSION is required, for example make release-dispatch VERSION=1.2.3"; exit 2 ;; esac
	@./scripts/dispatch-release.sh "$(VERSION)" "$(REPO)"

release-status: ## Inspect GitHub release/workflows/image: make release-status VERSION=1.2.3
	@case "$(VERSION)" in dev|0.0.0-dev.*) echo "VERSION is required, for example make release-status VERSION=1.2.3"; exit 2 ;; esac
	@./scripts/release-status.sh "$(VERSION)" "$(REPO)" "$(BACKEND_IMAGE)"

verify-image: ## Verify backend Docker image metadata: make verify-image IMAGE=repo/image:tag VERSION=1.2.3 [COMMIT=sha]
	@test -n "$(IMAGE)" || { echo "IMAGE is required, for example make verify-image IMAGE=ghcr.io/mohuganda/moh-sso-dashboard-backend:1.2.3 VERSION=1.2.3"; exit 2; }
	@test -n "$(VERSION)" || { echo "VERSION is required, for example make verify-image IMAGE=... VERSION=1.2.3"; exit 2; }
	@../infra/scripts/verify-backend-image.sh "$(IMAGE)" "$(VERSION)" "$(COMMIT)"

# ============================================================
#        🐳 Docker Commands
# ============================================================

postgres: ## Start the local development Postgres dependency
	$(MAKE) -C .. compose-dev-up SERVICE=backend-db

createdb: ## Create DB_NAME in the selected stack
	sh ../infra/scripts/compose.sh "$(COMPOSE_ENV)" exec -T backend-db sh -ec 'createdb -U "$$POSTGRES_USER" "$(DB_NAME)"'

backupdb: ## Backup the selected stack's database to backup.sql
	sh ../infra/scripts/compose.sh "$(COMPOSE_ENV)" exec -T backend-db sh -ec 'pg_dump -U "$$POSTGRES_USER" "$$POSTGRES_DB"' > backup.sql

restoredb: ## Restore backup.sql into the selected stack
	sh ../infra/scripts/compose.sh "$(COMPOSE_ENV)" exec -T backend-db sh -ec 'psql -U "$$POSTGRES_USER" "$$POSTGRES_DB"' < backup.sql

docker-up: ## Start docker-compose services
	@echo "🟢 Starting Docker containers..."
	$(MAKE) -C .. compose-up COMPOSE_ENV="$(COMPOSE_ENV)"

docker-down: ## Stop docker-compose services
	@echo "🔴 Stopping Docker containers..."
	$(MAKE) -C .. compose-down COMPOSE_ENV="$(COMPOSE_ENV)"

docker-logs: ## View logs from docker containers
	$(MAKE) -C .. compose-logs COMPOSE_ENV="$(COMPOSE_ENV)"

docker-restart: ## Restart docker services
	$(MAKE) -C .. compose-restart COMPOSE_ENV="$(COMPOSE_ENV)"

# ============================================================
#        🗄️ Database Migrations
# ============================================================

create_migration: ## Create a new migration file: make create_migration name=initial_schema
	migrate create -ext sql -dir "$(MIGRATE_PATH)" -seq $(name)

migrateup: ## Apply all up migrations
	migrate --path "$(MIGRATE_PATH)" --database "$(DB_URL)" --verbose up

migrateup1: ## Apply one up migration
	migrate --path "$(MIGRATE_PATH)" --database "$(DB_URL)" --verbose up 1

migratedown: ## Rollback all migrations
	migrate --path "$(MIGRATE_PATH)" --database "$(DB_URL)" --verbose down

migratedown1: ## Rollback one migration
	migrate --path "$(MIGRATE_PATH)" --database "$(DB_URL)" --verbose down 1

# ============================================================
# Make everything phony
# ============================================================
.PHONY: help tidy fmt lint test architecture build build-release build-cli run clean version verify-version \
        next-version release-check release-pr release-dispatch release-status verify-image \
        postgres createdb backupdb restoredb \
        docker-up docker-down docker-logs docker-restart \
        create_migration migrateup migrateup1 migratedown migratedown1 \
        deps verify
