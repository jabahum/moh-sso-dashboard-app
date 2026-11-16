# syntax=docker/dockerfile:1
# ======================
# Stage 1: Build
# ======================
FROM --platform=$BUILDPLATFORM golang:1.25.4-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown
ARG DIRTY=false

ENV CGO_ENABLED=0 GOWORK=off

WORKDIR /app

COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download && go mod verify

COPY backend/ .

RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
  GOOS="$TARGETOS" GOARCH="$TARGETARCH" go build \
  -trimpath \
  -ldflags="-s -w \
    -X 'github.com/moh-sso-dashboard/internal/version.Version=${VERSION}' \
    -X 'github.com/moh-sso-dashboard/internal/version.Commit=${COMMIT}' \
    -X 'github.com/moh-sso-dashboard/internal/version.BuildTime=${BUILD_TIME}' \
    -X 'github.com/moh-sso-dashboard/internal/version.Dirty=${DIRTY}'" \
  -o sso-server ./cmd/server

# ======================
# Stage 2: Runtime
# ======================
FROM alpine:3.19

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown
ARG SOURCE_URL=https://github.com/mohuganda/moh-sso-dashboard

LABEL org.opencontainers.image.title="MOH SSO Dashboard Backend" \
  org.opencontainers.image.version="$VERSION" \
  org.opencontainers.image.revision="$COMMIT" \
  org.opencontainers.image.created="$BUILD_TIME" \
  org.opencontainers.image.source="$SOURCE_URL"

RUN apk --no-cache add ca-certificates tzdata su-exec

WORKDIR /app

COPY --from=builder /app/internal/db/migrations ./internal/db/migrations
COPY --from=builder /app/internal/db/surveillance_migrations ./internal/db/surveillance_migrations
COPY --from=builder /app/config ./config
# The GeoJSON handler resolves ./assets/geojson from the runtime WORKDIR.
COPY --from=builder --chmod=644 /app/assets/geojson/*.geojson ./assets/geojson/
COPY --from=builder /app/sso-server ./sso-server

COPY --chmod=755 infra/docker/backend/entrypoint.sh /usr/local/bin/sso-entrypoint
RUN chmod 755 /app/assets /app/assets/geojson \
  && adduser -D appuser && mkdir -p /data/uploads && chown appuser:appuser /data/uploads
# The entrypoint reads restricted mounted secrets, then execs the server as appuser.
ENV SSO_RUNTIME_USER=appuser

EXPOSE 9000

ENTRYPOINT ["/usr/local/bin/sso-entrypoint", "/app/sso-server"]
CMD []
