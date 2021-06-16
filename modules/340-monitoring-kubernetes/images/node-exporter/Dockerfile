# Based on https://github.com/prometheus/node_exporter/blob/v0.18.1/Dockerfile
ARG BASE_ALPINE
FROM prom/node-exporter:v0.18.1@sha256:a2f29256e53cc3e0b64d7a472512600b2e9410347d53cdc85b49f659c17e02ee as artifact

FROM $BASE_ALPINE
COPY --from=artifact /bin/node_exporter /bin

EXPOSE      9100
USER        nobody
ENTRYPOINT  [ "/bin/node_exporter" ]
