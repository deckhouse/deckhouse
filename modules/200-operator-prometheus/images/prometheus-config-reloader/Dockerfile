# Based on https://github.com/prometheus-operator/prometheus-operator/blob/v0.31.1/cmd/prometheus-config-reloader/Dockerfile
ARG BASE_ALPINE
FROM quay.io/coreos/prometheus-config-reloader:v0.31.1@sha256:5e508e2e0aba161359caad284b6974fa11119a55c2d5786d024ad32cffc61634 as artifact

FROM $BASE_ALPINE

COPY --from=artifact /bin/prometheus-config-reloader /bin/

RUN chown nobody:nogroup /bin/prometheus-config-reloader

USER nobody

ENTRYPOINT ["/bin/prometheus-config-reloader"]
