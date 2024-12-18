FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder

ARG BUILD_TAGS=log_plain

ENV APP_PATH_FROM=./ee/modules/038-system-registry/images/mirrorer/app

# Create tmp dir
RUN mkdir -m 1777 /tmp-tmp

# Copy go.mod and go.sum
RUN mkdir -p /src/
COPY $APP_PATH_FROM/go.mod $APP_PATH_FROM/go.sum /src/

# Download libs
RUN cd /src/ && go mod download -x

# Copy other files
COPY $APP_PATH_FROM/ /src/

# Build binary
ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    cd /src/ && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 go build -tags "${BUILD_TAGS}" -o /mirrorer ./cmd/mirrorer && \
    chown 64535:64535 /mirrorer && \
    chmod 0755 /mirrorer

FROM --platform=linux/amd64 alpine:3.20
RUN apk add --no-cache iproute2 curl vim bash

COPY --from=builder /tmp-tmp /tmp
COPY --from=builder /mirrorer /mirrorer

ENTRYPOINT ["/mirrorer"]
