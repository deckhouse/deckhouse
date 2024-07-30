ARG BASE_ALPINE
ARG BASE_GOLANG_20_ALPINE_DEV
ARG BASE_DISTROLESS
FROM $BASE_GOLANG_20_ALPINE_DEV as artifact
WORKDIR /src/
COPY / /src/
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags='-extldflags "-static" -s -w' -o ./protobuf_exporter ./main.go

FROM $BASE_DISTROLESS
COPY --from=artifact /src/protobuf_exporter /protobuf_exporter
COPY rootfs /
ENTRYPOINT [ "/protobuf_exporter", "-server.telemetry-address", "127.0.0.1:9090", "-server.exporter-address", "127.0.0.1:9091" , "-mappings", "/etc/protobuf_exporter/mappings.yaml"]
