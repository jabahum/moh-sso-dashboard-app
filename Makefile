# -----------------------------
# Project Metadata
# -----------------------------
PROJECT_NAME = moh-sso-dashboard
GO_CMD = go
GO_MAIN = ./cmd/server/main.go
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
	cd $(FRONTEND_DIR) && npm install

.PHONY: frontend-build
frontend-build:
	@echo "🏗️ Building React app..."
	cd $(FRONTEND_DIR) && npm run build

.PHONY: frontend-start
frontend-start:
	@echo "🌐 Starting React development server..."
	cd $(FRONTEND_DIR) && npm start

# -----------------------------
# Docker Commands
# -----------------------------
.PHONY: docker-build
docker-build:
	@echo "🐳 Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .

.PHONY: docker-run
docker-run:
	@echo "🐳 Running Docker container..."
	docker run -p 8080:8080 $(DOCKER_IMAGE)

# -----------------------------
# Docker Compose Commands
# -----------------------------
.PHONY: compose-up
compose-up:
	@echo "📦 Starting services with Docker Compose..."
	docker compose up -d

.PHONY: compose-down
compose-down:
	@echo "🧯 Stopping services..."
	docker compose down

.PHONY: backend-dev
backend-dev:
	@echo "⚡ Starting the backend with automatic reload..."
	docker compose -f docker-compose.dev.yml up --build backend

.PHONY: backend-dev-logs
backend-dev-logs:
	docker compose -f docker-compose.dev.yml logs -f backend

# -----------------------------
# Utility Commands
# -----------------------------
.PHONY: dev
dev: sso-up

.PHONY: sso-up sso-down sso-status sso-reconcile-keycloak dev-native
sso-up:
	python3 scripts/local-sso.py up

sso-down:
	python3 scripts/local-sso.py down

sso-status:
	python3 scripts/local-sso.py status

sso-reconcile-keycloak:
	python3 scripts/local-sso.py reconcile-keycloak

dev-native:
	@echo "Starting native backend and frontend (infrastructure managed separately)..."
	$(MAKE) -j2 run frontend-start

.PHONY: all
all: tidy build frontend-build
	@echo "✅ All components built successfully!"

# -----------------------------
# Kubernetes Local (Docker Desktop)
# -----------------------------

K8S_NAMESPACE = sso-local
HELM_RELEASE = moh-sso-dashboard
HELM_CHART = ./charts/moh-sso
VALUES_LOCAL = charts/moh-sso/values-local.yaml
VALUES_DEV = charts/moh-sso/values-dev.yaml
VALUES_PROD = charts/moh-sso/values-prod.yaml

BACKEND_IMAGE = moh-sso-dashboard-backend:local
FRONTEND_IMAGE = moh-sso-dashboard-frontend:local

.PHONY: local-namespace
local-namespace:
	@echo "📦 Ensuring namespace exists..."
	kubectl get namespace $(K8S_NAMESPACE) >/dev/null 2>&1 || kubectl create namespace $(K8S_NAMESPACE)

.PHONY: local-build
local-build:
	@echo "🐳 Building backend (Dockerfile.dev) for arm64..."
	docker buildx build \
		--platform linux/arm64 \
		-f backend/Dockerfile.dev \
		-t $(BACKEND_IMAGE) \
		--load \
		./backend
	@echo "🐳 Building frontend (Dockerfile.dev) for arm64..."
	docker buildx build \
		--platform linux/arm64 \
		-f frontend/Dockerfile.dev \
		-t $(FRONTEND_IMAGE) \
		--load \
		./frontend
	@echo "✅ Local arm64 images built and loaded into containerd"


.PHONY: local-up
local-up: local-namespace
	@echo "🚀 Deploying to Kubernetes (local)..."
	helm upgrade --install $(HELM_RELEASE) $(HELM_CHART) \
		-n $(K8S_NAMESPACE) \
		-f $(VALUES_LOCAL)
	@echo "✅ Deployment applied"

.PHONY: local-down
local-down:
	@echo "🧯 Uninstalling local release..."
	helm uninstall $(HELM_RELEASE) -n $(K8S_NAMESPACE) || true

.PHONY: local-restart
local-restart:
	@echo "🔄 Restarting backend & frontend..."
	kubectl rollout restart deployment $(HELM_RELEASE)-backend -n $(K8S_NAMESPACE)
	kubectl rollout restart deployment $(HELM_RELEASE)-frontend -n $(K8S_NAMESPACE)

.PHONY: local-logs
local-logs:
	kubectl get pods -n $(K8S_NAMESPACE)

.PHONY: local-backend-logs
local-backend-logs:
	kubectl logs -f deployment/$(HELM_RELEASE)-backend -n $(K8S_NAMESPACE)

.PHONY: local-keycloak-logs
local-keycloak-logs:
	kubectl logs -f statefulset/$(HELM_RELEASE)-keycloak -n $(K8S_NAMESPACE)

.PHONY: local-forward
local-forward:
	@echo "🌐 Port forwarding..."
	@echo "Frontend:  http://localhost:3000"
	@echo "Backend:   http://localhost:9000"
	@echo "Keycloak:  http://localhost:8081"
	kubectl port-forward svc/$(HELM_RELEASE)-frontend 3000:80 -n $(K8S_NAMESPACE) & \
	kubectl port-forward svc/$(HELM_RELEASE)-backend 9000:9000 -n $(K8S_NAMESPACE) & \
	kubectl port-forward svc/$(HELM_RELEASE)-keycloak 8081:8080 -n $(K8S_NAMESPACE)

.PHONY: local
local: local-build local-up
	@echo "🎉 Local stack deployed!"
	@echo "Run 'make local-forward' to access services."

.PHONY: helm-lint
helm-lint:
	@echo "🔎 Linting Helm chart..."
	helm lint $(HELM_CHART)

.PHONY: helm-template-local
helm-template-local:
	@echo "🧾 Rendering local Helm values..."
	helm template $(HELM_RELEASE) $(HELM_CHART) -f $(VALUES_LOCAL)

.PHONY: helm-template-dev
helm-template-dev:
	@echo "🧾 Rendering dev Helm values..."
	helm template $(HELM_RELEASE) $(HELM_CHART) -f $(VALUES_DEV)

.PHONY: helm-template-prod
helm-template-prod:
	@test -n "$(BACKEND_TAG)" || (echo "BACKEND_TAG is required for helm-template-prod" && exit 1)
	@test -n "$(FRONTEND_TAG)" || (echo "FRONTEND_TAG is required for helm-template-prod" && exit 1)
	@echo "🧾 Rendering prod Helm values..."
	helm template $(HELM_RELEASE) $(HELM_CHART) -f $(VALUES_PROD) \
		--set backend.image.tag=$(BACKEND_TAG) \
		--set frontend.image.tag=$(FRONTEND_TAG)

.PHONY: helm-check
helm-check: helm-lint helm-template-local helm-template-dev
	@echo "✅ Helm chart checks passed"

.PHONY: helm-check-prod
helm-check-prod: helm-lint helm-template-prod
	@echo "✅ Helm production chart checks passed"

# -----------------------------
# Kubernetes DEV (Cluster)
# -----------------------------

DEV_NAMESPACE = sso-dev
.PHONY: dev-up
dev-up:
	@echo "🚀 Deploying DEV environment..."
	helm upgrade --install $(HELM_RELEASE) $(HELM_CHART) \
		-n $(DEV_NAMESPACE) \
		-f $(DEV_VALUES)

.PHONY: dev-down
dev-down:
	@echo "🧯 Removing DEV release..."
	helm uninstall $(HELM_RELEASE) -n $(DEV_NAMESPACE) || true

.PHONY: dev-restart
dev-restart:
	@echo "🔄 Restarting backend & frontend..."
	kubectl rollout restart deployment $(HELM_RELEASE)-backend -n $(DEV_NAMESPACE)
	kubectl rollout restart deployment $(HELM_RELEASE)-frontend -n $(DEV_NAMESPACE)

.PHONY: dev-logs
dev-logs:
	kubectl get pods -n $(DEV_NAMESPACE)

.PHONY: dev-logs-watch
dev-logs-watch:
	kubectl get pods -n $(DEV_NAMESPACE) -w

.PHONY: dev-backend-logs
dev-backend-logs:
	kubectl logs -f deployment/$(HELM_RELEASE)-backend -n $(DEV_NAMESPACE)

.PHONY: dev-frontend-logs
dev-frontend-logs:
	kubectl logs -f deployment/$(HELM_RELEASE)-frontend -n $(DEV_NAMESPACE)

# -----------------------------
# Kubernetes PROD
# -----------------------------

PROD_NAMESPACE = sso-prod
.PHONY: prod-up
prod-up:
	@echo "🚀 Deploying PROD environment..."
	@test -n "$(BACKEND_TAG)" || (echo "BACKEND_TAG is required for prod-up" && exit 1)
	@test -n "$(FRONTEND_TAG)" || (echo "FRONTEND_TAG is required for prod-up" && exit 1)
	helm upgrade --install $(HELM_RELEASE) $(HELM_CHART) \
		-n $(PROD_NAMESPACE) \
		-f $(VALUES_PROD) \
		--set backend.image.tag=$(BACKEND_TAG) \
		--set frontend.image.tag=$(FRONTEND_TAG)

.PHONY: prod-down
prod-down:
	@echo "🧯 Removing PROD release..."
	helm uninstall $(HELM_RELEASE) -n $(PROD_NAMESPACE) || true


.PHONY: prod-restart
prod-restart:
	@echo "🔄 Restarting backend & frontend..."
	kubectl rollout restart deployment $(HELM_RELEASE)-backend -n $(PROD_NAMESPACE)
	kubectl rollout restart deployment $(HELM_RELEASE)-frontend -n $(PROD_NAMESPACE)

.PHONY: prod-logs
prod-logs:
	kubectl get pods -n $(PROD_NAMESPACE)

.PHONY: prod-backend-logs
prod-backend-logs:
	kubectl logs -f deployment/$(HELM_RELEASE)-backend -n $(PROD_NAMESPACE)

.PHONY: prod-frontend-logs
prod-frontend-logs:
	kubectl logs -f deployment/$(HELM_RELEASE)-frontend -n $(PROD_NAMESPACE)
