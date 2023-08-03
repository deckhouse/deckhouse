ARG BASE_ALPINE
ARG BASE_GOLANG_20_ALPINE

FROM $BASE_GOLANG_20_ALPINE as artifact

WORKDIR /src
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY internal/ internal
# Copy the bash source
COPY bash/cleanup_static_node.sh cleanup_static_node.sh

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager cmd/main.go

FROM $BASE_ALPINE
WORKDIR /

COPY --from=artifact /src/manager /
COPY --from=artifact /src/cleanup_static_node.sh /

RUN apk add --update --no-cache openssh

USER 65534:65534

ENTRYPOINT [ "/manager" ]
