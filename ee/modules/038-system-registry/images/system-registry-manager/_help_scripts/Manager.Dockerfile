FROM registry.k8s.io/pause:latest as pause

FROM --platform=$BUILDPLATFORM golang:1.24-alpine3.20 AS builder

ARG BUILD_TAGS=log_plain

ENV APP_PATH_FROM=./ee/modules/038-system-registry/images/system-registry-manager/app \
    APP_PATH_TO=/deckhouse/ee/modules/038-system-registry/images/system-registry-manager/app \
    GO_LIB_PATH_FROM=./go_lib/system-registry-manager \
    GO_LIB_PATH_TO=/deckhouse/go_lib/system-registry-manager

# Set GOPROXY and GOSUMDB
#ENV GOPROXY=http://10.211.55.2:8081/repository/golang-proxy
#ENV GOSUMDB='sum.golang.org http://10.211.55.2:8081/repository/golang-sum-proxy'

# Create tmp dir
RUN mkdir -m 1777 /tmp-tmp

# Copy go.mod and go.sum
RUN mkdir -p ${APP_PATH_TO} ${GO_LIB_PATH_TO} ${LOGGER_PATH_TO}
COPY ${APP_PATH_FROM}/go.mod ${APP_PATH_FROM}/go.sum ${APP_PATH_TO}/
COPY ${GO_LIB_PATH_FROM}/go.mod ${GO_LIB_PATH_FROM}/go.sum ${GO_LIB_PATH_TO}/

# Download libs
RUN cd ${APP_PATH_TO} && go mod download -x && \
    cd ${GO_LIB_PATH_TO} && go mod download -x

# Copy other files
COPY ${APP_PATH_FROM}/ ${APP_PATH_TO}/
COPY ${GO_LIB_PATH_FROM}/ ${GO_LIB_PATH_TO}/

# Run tests
RUN --mount=type=cache,target=/root/.cache/go-build \
  cd ${APP_PATH_TO} && \
  GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go test -tags "${BUILD_TAGS}" ./...

# Build binary
ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    cd ${APP_PATH_TO} && \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -tags "${BUILD_TAGS}" -o /manager ./cmd/manager && \
    chown 64535:64535 /manager && \
    chmod 0755 /manager

RUN --mount=type=cache,target=/root/.cache/go-build \
    cd ${APP_PATH_TO} && \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -tags "${BUILD_TAGS}" -o /staticpod ./cmd/staticpod && \
    chown 64535:64535 /staticpod && \
    chmod 0755 /staticpod

## Install delve
#RUN GOARCH=amd64 go install github.com/go-delve/delve/cmd/dlv@latest
#RUN mkdir -p /tmp-tmp/dlv && chmod -R 777 /tmp-tmp/dlv

# Copy binary and dlv into new container
#FROM --platform=linux/amd64 scratch
FROM --platform=linux/amd64 alpine:3.20
RUN apk add --no-cache iproute2 curl vim bash
ENV APP_PATH_FROM=./ee/modules/038-system-registry/images/system-registry-manager
COPY --from=builder /tmp-tmp /tmp
COPY --from=builder /manager /manager
COPY --from=builder /staticpod /staticpod
COPY --from=pause /pause /pause

#COPY --from=builder /go/bin/linux_amd64/dlv /dlv
#ENV XDG_CONFIG_HOME=/tmp/dlv

#ENTRYPOINT ["/dlv", "exec", "/manager", "--headless=true", "--listen=0.0.0.0:9876", "--api-version=2", "--accept-multiclient", "--continue", "--"]
# Usage example kubectl port-forward pod/system-registry-manager-jzw4r 9876:9876 -n d8-system
ENTRYPOINT ["/manager"]
