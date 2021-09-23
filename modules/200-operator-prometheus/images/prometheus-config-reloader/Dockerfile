# Based on https://github.com/prometheus-operator/prometheus-operator/blob/v0.50.0/cmd/prometheus-config-reloader/Dockerfile
ARG BASE_ALPINE
FROM quay.io/prometheus-operator/prometheus-config-reloader:v0.50.0@sha256:8b42df399f6d8085d9c7377e4f5508c10791d19cd1df00ab41de856741c65d28 as artifact

FROM $BASE_ALPINE

COPY --from=artifact /bin/prometheus-config-reloader /bin/

RUN chown nobody:nogroup /bin/prometheus-config-reloader

USER nobody

ENTRYPOINT ["/bin/prometheus-config-reloader"]
