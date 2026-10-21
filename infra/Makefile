# Deployment commands shared by the root and infra Makefiles.
INFRA_MAKEFILE := $(lastword $(MAKEFILE_LIST))
REPO_ROOT := $(abspath $(dir $(INFRA_MAKEFILE))/..)
PYTHON ?= python3
NODE ?= node
DOCKER ?= docker
COMPOSE_ENV ?= prod
ENV_FILE ?=
REMOTE_ENV_FILE ?=
SERVICE ?=
COMPOSE = ENV_FILE="$(ENV_FILE)" REMOTE_ENV_FILE="$(REMOTE_ENV_FILE)" sh "$(REPO_ROOT)/infra/scripts/compose.sh" "$(COMPOSE_ENV)"
BACKEND_VERSION ?= $(shell cat "$(REPO_ROOT)/backend/VERSION")
FRONTEND_VERSION ?= $(shell $(NODE) -p "require('$(REPO_ROOT)/frontend/apps/shell/package.json').version")
BUILD_PLATFORM ?= linux/arm64
ifndef INFRA_EMBEDDED
.DEFAULT_GOAL := infra-help
endif

.PHONY: infra-help compose-config compose-up compose-down compose-logs compose-ps compose-pull compose-restart \
 compose-dev-up compose-prod-up compose-gateway-up compose-remote-up compose-external-up \
 infra-check infra-runtime-check infra-nginx-check infra-test docker-build docker-run backend-dev backend-dev-logs \
 sso-up sso-down sso-status sso-reconcile-keycloak
infra-help:
	@echo 'Deployment commands (COMPOSE_ENV defaults to prod):'
	@echo '  make compose-config                        Validate configuration; starts nothing'
	@echo '  make compose-up                            Start/reconcile in background [SERVICE=backend]'
	@echo '  make compose-down                          Stop/remove selected stack; keep data volumes'
	@echo '  make compose-logs                          Follow logs [SERVICE=backend]'
	@echo '  make compose-ps                            Show selected stack status'
	@echo '  make compose-pull                          Download images [SERVICE=backend]'
	@echo '  make compose-restart                       Restart existing containers [SERVICE=backend]'
	@echo '  Modes: COMPOSE_ENV=prod, dev, gateway, remote, or external (choose one)'
	@echo ''
	@echo 'Start shortcuts (each fixes its own mode; all accept SERVICE):'
	@echo '  make compose-dev-up                        Build and start local development'
	@echo '  make compose-prod-up                       Start production image tags'
	@echo '  make compose-gateway-up                    Start production with container Nginx'
	@echo '  make compose-remote-up                     Start production for remote Nginx'
	@echo '  make compose-external-up                   Build/start development with external Keycloak'
	@echo ''
	@echo 'Checks:'
	@echo '  make infra-check                           Validate Compose/env/path/launcher contracts'
	@echo '  make infra-runtime-check                   Test container startup with disposable fixtures'
	@echo '  make infra-nginx-check                     Test gateway, host and remote-IP routing'
	@echo '  make infra-test                            Run all three infrastructure checks'
	@echo ''
	@echo 'ENV_FILE overrides the main env file; REMOTE_ENV_FILE selects the remote overlay.'
	@echo 'Defaults: infra/env/{prod,dev,external-dev,remote-proxy}.env; dev has an example fallback.'
	@echo 'Override paths are relative to the repo root or absolute. Env files are never sourced.'
	@echo 'Guide: docs/make.md (prerequisites, examples, generated files and command effects).'

compose-config:
	$(COMPOSE) config --quiet
compose-up:
	$(COMPOSE) up -d $(SERVICE)
compose-down:
	$(COMPOSE) down
compose-logs:
	$(COMPOSE) logs -f $(SERVICE)
compose-ps:
	$(COMPOSE) ps
compose-pull:
	$(COMPOSE) pull $(SERVICE)
compose-restart:
	$(COMPOSE) restart $(SERVICE)
compose-dev-up:
	ENV_FILE="$(ENV_FILE)" sh "$(REPO_ROOT)/infra/scripts/compose.sh" dev up --build -d $(SERVICE)
compose-prod-up:
	ENV_FILE="$(ENV_FILE)" sh "$(REPO_ROOT)/infra/scripts/compose.sh" prod up -d $(SERVICE)
compose-gateway-up:
	ENV_FILE="$(ENV_FILE)" sh "$(REPO_ROOT)/infra/scripts/compose.sh" gateway up -d $(SERVICE)
compose-remote-up:
	ENV_FILE="$(ENV_FILE)" REMOTE_ENV_FILE="$(REMOTE_ENV_FILE)" sh "$(REPO_ROOT)/infra/scripts/compose.sh" remote up -d $(SERVICE)
compose-external-up:
	ENV_FILE="$(ENV_FILE)" sh "$(REPO_ROOT)/infra/scripts/compose.sh" external up --build -d $(SERVICE)
backend-dev:
	ENV_FILE="$(ENV_FILE)" sh "$(REPO_ROOT)/infra/scripts/compose.sh" dev up --build backend
backend-dev-logs:
	ENV_FILE="$(ENV_FILE)" sh "$(REPO_ROOT)/infra/scripts/compose.sh" dev logs -f backend
infra-check:
	$(PYTHON) "$(REPO_ROOT)/infra/scripts/test-compose.py"
infra-runtime-check:
	$(PYTHON) "$(REPO_ROOT)/infra/scripts/test-compose-runtime.py"
infra-nginx-check:
	$(PYTHON) "$(REPO_ROOT)/infra/scripts/test-nginx.py"
infra-test: infra-check infra-runtime-check infra-nginx-check

docker-build:
	$(DOCKER) build -f "$(REPO_ROOT)/infra/docker/backend/Dockerfile" --build-arg VERSION="$(BACKEND_VERSION)" -t moh-sso-dashboard-backend:local "$(REPO_ROOT)"
	$(DOCKER) build -f "$(REPO_ROOT)/infra/docker/frontend/Dockerfile" --build-arg VERSION="$(FRONTEND_VERSION)" -t moh-sso-dashboard-frontend:local "$(REPO_ROOT)"
docker-run: compose-up

sso-up:
	$(PYTHON) "$(REPO_ROOT)/infra/scripts/local-sso.py" up
sso-down:
	$(PYTHON) "$(REPO_ROOT)/infra/scripts/local-sso.py" down
sso-status:
	$(PYTHON) "$(REPO_ROOT)/infra/scripts/local-sso.py" status
sso-reconcile-keycloak:
	$(PYTHON) "$(REPO_ROOT)/infra/scripts/local-sso.py" reconcile-keycloak

# -----------------------------
# Kubernetes Local (Docker Desktop)
# -----------------------------

K8S_NAMESPACE = sso-local
HELM_RELEASE = moh-sso-dashboard
HELM_CHART = $(REPO_ROOT)/infra/helm/moh-sso
VALUES_LOCAL = $(REPO_ROOT)/infra/helm/moh-sso/values-local.yaml
VALUES_DEV = $(REPO_ROOT)/infra/helm/moh-sso/values-dev.yaml
VALUES_PROD = $(REPO_ROOT)/infra/helm/moh-sso/values-prod.yaml

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
		--platform $(BUILD_PLATFORM) \
		-f $(REPO_ROOT)/infra/docker/backend/Dockerfile.dev \
		-t $(BACKEND_IMAGE) \
		--load \
		$(REPO_ROOT)
	@echo "🐳 Building frontend (Dockerfile.dev) for arm64..."
	docker buildx build \
		--platform $(BUILD_PLATFORM) \
		-f $(REPO_ROOT)/infra/docker/frontend/Dockerfile.dev \
		-t $(FRONTEND_IMAGE) \
		--load \
		$(REPO_ROOT)
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
		-f $(VALUES_DEV)

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
