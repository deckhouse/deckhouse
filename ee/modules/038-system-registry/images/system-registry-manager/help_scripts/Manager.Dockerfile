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
    GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 go build -ldflags="-s -w" -o /manager ./cmd/manager && \
    chown 64535:64535 /manager && \
    chmod 0755 /manager


# Copy binary to new container
FROM --platform=$BUILDPLATFORM scratch
ENV MANAGER_PATH_FROM=./ee/modules/038-system-registry/images/system-registry-manager
COPY $MANAGER_PATH_FROM/templates /templates
COPY --from=builder /tmp-tmp /tmp
COPY --from=builder /manager /manager
ENTRYPOINT ["/manager"]
