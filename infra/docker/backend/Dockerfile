# ======================
# Stage 1: Build
# ======================
FROM --platform=$BUILDPLATFORM golang:1.25.4-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown
ARG DIRTY=false

ENV CGO_ENABLED=0

WORKDIR /app

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .

RUN GOOS="$TARGETOS" GOARCH="$TARGETARCH" go build \
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
COPY --from=builder /app/config ./config
COPY --from=builder /app/sso-server ./sso-server

COPY --chmod=755 infra/docker/backend/entrypoint.sh /usr/local/bin/sso-entrypoint
RUN adduser -D appuser && mkdir -p /data/uploads && chown appuser:appuser /data/uploads
# The entrypoint reads restricted mounted secrets, then execs the server as appuser.
ENV SSO_RUNTIME_USER=appuser

EXPOSE 9000

ENTRYPOINT ["/usr/local/bin/sso-entrypoint", "/app/sso-server"]
CMD []
