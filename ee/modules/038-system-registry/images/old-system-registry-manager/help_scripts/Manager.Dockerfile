FROM --platform=$BUILDPLATFORM golang:1.22-bookworm AS builder

ENV MANAGER_PATH_FROM=./ee/modules/038-system-registry/images/system-registry-manager/manager \
    MANAGER_PATH_TO=/deckhouse/ee/modules/038-system-registry/images/system-registry-manager/manager \
    GO_LIB_PATH_FROM=./go_lib/system-registry-manager \
    GO_LIB_PATH_TO=/deckhouse/go_lib/system-registry-manager


# Create tmp dir
RUN mkdir -m 1777 /tmp-tmp

# Copy go.mod and go.sum
RUN mkdir -p $MANAGER_PATH_TO $GO_LIB_PATH_TO
COPY $MANAGER_PATH_FROM/go.mod $MANAGER_PATH_FROM/go.sum $MANAGER_PATH_TO/
COPY $GO_LIB_PATH_FROM/go.mod $GO_LIB_PATH_FROM/go.sum $GO_LIB_PATH_TO/

# Download libs
RUN --mount=type=cache,target=/go/pkg/mod \
    cd $MANAGER_PATH_TO && go mod download && \
    cd $GO_LIB_PATH_TO && go mod download

# Copy other files
RUN mkdir -p $MANAGER_PATH_TO $GO_LIB_PATH_TO
COPY $MANAGER_PATH_FROM/ $MANAGER_PATH_TO/
COPY $GO_LIB_PATH_FROM/ $GO_LIB_PATH_TO/

# Build binary
ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    cd $MANAGER_PATH_TO && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 go build -gcflags "all=-N -l" -o /manager ./cmd/manager && \
    chown 64535:64535 /manager && \
    chmod 0755 /manager

# Install delve
RUN GOARCH=amd64 go install github.com/go-delve/delve/cmd/dlv@latest
RUN mkdir -p /tmp-tmp/dlv && chmod -R 777 /tmp-tmp/dlv

# Copy binary, templates and dlv into new container
FROM --platform=$BUILDPLATFORM scratch
ENV MANAGER_PATH_FROM=./ee/modules/038-system-registry/images/system-registry-manager
COPY $MANAGER_PATH_FROM/templates /templates
COPY --from=builder /tmp-tmp /tmp
COPY --from=builder /manager /manager
COPY --from=builder /go/bin/linux_amd64/dlv /dlv
ENV XDG_CONFIG_HOME=/tmp/dlv

ENTRYPOINT ["/dlv", "exec", "/manager", "--headless=true", "--listen=0.0.0.0:9876", "--api-version=2", "--accept-multiclient", "--continue", "--"]
# Usage example kubectl port-forward pod/system-registry-manager-jzw4r 9876:9876 -n d8-system
