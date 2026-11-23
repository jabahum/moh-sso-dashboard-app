# syntax=docker/dockerfile:1
# E2E-only image: the same build as infra/docker/backend/Dockerfile, plus the
# moh-sso CLI so one-shot containers can run "identity-sync register|describe".
# Build context: the repository root.
FROM golang:1.25.4-alpine AS builder
ENV CGO_ENABLED=0 GOWORK=off
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
COPY backend/core/go.mod ./core/go.mod
RUN --mount=type=cache,target=/go/pkg/mod go mod download && go mod verify
COPY backend/ .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
  go build -trimpath -ldflags="-s -w" -o sso-server ./cmd/server && \
  go build -trimpath -ldflags="-s -w" -o moh-sso ./cmd/cli

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata su-exec
WORKDIR /app
COPY --from=builder /app/internal/db/migrations ./internal/db/migrations
COPY --from=builder /app/internal/db/surveillance_migrations ./internal/db/surveillance_migrations
COPY --from=builder /app/config ./config
COPY --from=builder /app/sso-server /app/moh-sso ./
COPY --chmod=755 infra/docker/backend/entrypoint.sh /usr/local/bin/sso-entrypoint
RUN adduser -D appuser && mkdir -p /data/uploads && chown appuser:appuser /data/uploads
ENV SSO_RUNTIME_USER=appuser
EXPOSE 9000
ENTRYPOINT ["/usr/local/bin/sso-entrypoint", "/app/sso-server"]
