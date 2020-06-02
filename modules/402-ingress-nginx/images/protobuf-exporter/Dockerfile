FROM golang:1.13-alpine3.10
WORKDIR /src/
COPY / /src/
RUN apk add --no-cache git && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ./protobuf_exporter ./main.go

FROM alpine:3.10.2
COPY --from=0 /src/protobuf_exporter /protobuf_exporter
COPY rootfs /
ENTRYPOINT [ "/protobuf_exporter", "-server.telemetry-address", "127.0.0.1:9090", "-server.exporter-address", "127.0.0.1:9091" , "-mappings", "/etc/protobuf_exporter/mappings.yaml"]
