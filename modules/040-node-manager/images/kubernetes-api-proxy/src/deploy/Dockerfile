ARG GO_VERSION=1.25

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS builder

ENV CGO_ENABLED=0

WORKDIR /src

COPY ../go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY .. .

# Build (Linux-only)
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/kubernetes-api-proxy ./cmd/kubernetes-api-proxy


FROM scratch

LABEL org.opencontainers.image.title="kubernetes-api-proxy" \
      org.opencontainers.image.description="Lightweight TCP load balancer for Kubernetes API servers" \
      org.opencontainers.image.source="https://github.com/deckhouse/deckhouse" \
      org.opencontainers.image.licenses="Apache-2.0"

COPY --from=builder /out/kubernetes-api-proxy /kubernetes-api-proxy

USER 65532:65532

EXPOSE 6445/tcp 8080/tcp

ENTRYPOINT ["/kubernetes-api-proxy"]
