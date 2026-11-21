# syntax=docker/dockerfile:1
# ======================
# Stage 1: Build
# ======================
FROM node:22-alpine AS build

WORKDIR /app
ARG VERSION
ARG COMMIT=none
ARG BUILD_TIME
ARG FRONTEND_MODULE_SOURCE=""
ARG FRONTEND_MODULE_CONFIG=module-sources.json
ENV FRONTEND_MODULE_SOURCE=$FRONTEND_MODULE_SOURCE
ENV FRONTEND_MODULE_CONFIG=$FRONTEND_MODULE_CONFIG
ENV VITE_APP_VERSION=$VERSION
ENV VITE_BUILD_TIME=$BUILD_TIME
ENV FRONTEND_COMMIT=$COMMIT
ENV FRONTEND_ASSET_BASE_URL=/portal
ENV FRONTEND_BASE_PATH=/portal
ENV NODE_OPTIONS=--max-old-space-size=4096

COPY frontend/ .
RUN --mount=type=cache,target=/root/.npm npm ci --no-audit --no-fund --fetch-retries=5 --fetch-retry-mintimeout=20000
RUN npm run version:audit
RUN node scripts/use-runtime-config.mjs production
RUN npm run build:docker

# Build the guides with repository-relative links and images in scope.
FROM node:22-alpine AS documentation
WORKDIR /repository
COPY documentation/package.json documentation/package-lock.json ./documentation/
RUN npm ci --prefix documentation --no-audit --no-fund
COPY . .
ARG DOCS_URL=https://dashboards.health.go.ug
ENV DOCS_BASE_URL=/docs/
RUN npm --prefix documentation run build

# ======================
# Stage 2: Production (Static server)
# ======================
FROM nginx:1.30.6-alpine
RUN apk add --no-cache jq
COPY --chmod=755 infra/docker/frontend/40-runtime-config.sh /docker-entrypoint.d/40-runtime-config.sh

ARG VERSION
ARG COMMIT=none
ARG BUILD_TIME
ARG SOURCE_URL=https://github.com/mohuganda/moh-sso-dashboard
LABEL org.opencontainers.image.title="MOH SSO Dashboard Frontend" \
  org.opencontainers.image.version="$VERSION" \
  org.opencontainers.image.revision="$COMMIT" \
  org.opencontainers.image.created="$BUILD_TIME" \
  org.opencontainers.image.source="$SOURCE_URL"

COPY --from=build /app/dist /usr/share/nginx/html
COPY --from=documentation /repository/documentation/build /usr/share/nginx/html/docs
COPY infra/docker/frontend/nginx.conf /etc/nginx/conf.d/default.conf
# Preserve the published image's existing port contract.
RUN sed -i 's/listen 80 default_server;/listen 3000 default_server;/' /etc/nginx/conf.d/default.conf

EXPOSE 3000

CMD ["nginx", "-g", "daemon off;"]
