# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.mod
COPY go.sum go.sum

# Download dependencies
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY api/ api/
COPY internal/ internal/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o node-controller ./cmd/main.go

# Final stage
FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /workspace/node-controller .

USER 65532:65532

ENTRYPOINT ["/node-controller"]
