FROM registry.k8s.io/pause:latest as pause

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

ARG BUILD_TAGS=log_plain

ENV APP_PATH_FROM=./ee/modules/038-system-registry/images/mirrorer/app \
    APP_PATH_TO=/deckhouse/ee/modules/038-system-registry/images/mirrorer/app

# Create tmp dir
RUN mkdir -m 1777 /tmp-tmp

# Copy go.mod and go.sum
RUN mkdir -p ${APP_PATH_TO} ${LOGGER_PATH_TO}
COPY ${APP_PATH_FROM}/go.mod ${APP_PATH_FROM}/go.sum ${APP_PATH_TO}/

# Download libs
RUN cd ${APP_PATH_TO} && go mod download -x

# Copy other files
COPY ${APP_PATH_FROM}/ ${APP_PATH_TO}/

# Run tests
RUN --mount=type=cache,target=/root/.cache/go-build \
  cd ${APP_PATH_TO} && \
  GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go test -tags "${BUILD_TAGS}" ./...

# Build binary
ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    cd ${APP_PATH_TO} && \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -tags "${BUILD_TAGS}" -o /mirrorer ./cmd/mirrorer && \
    chown 64535:64535 /mirrorer && \
    chmod 0755 /mirrorer

FROM --platform=linux/amd64 alpine:3.20
RUN apk add --no-cache iproute2 curl vim bash

COPY --from=builder /tmp-tmp /tmp
COPY --from=builder /mirrorer /mirrorer
COPY --from=pause /pause /pause

ENTRYPOINT ["/mirrorer"]
