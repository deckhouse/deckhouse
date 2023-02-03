# Based on https://github.com/prometheus-operator/prometheus-operator/blob/v0.50.0/cmd/prometheus-config-reloader/Dockerfile
ARG BASE_ALPINE
FROM quay.io/prometheus-operator/prometheus-config-reloader:v0.62.0@sha256:267b12235d10a810e49b2d40a3deda6256f92690b26997a199601a47283821b9 as artifact

FROM $BASE_ALPINE

COPY --from=artifact /bin/prometheus-config-reloader /bin/

RUN chown nobody:nogroup /bin/prometheus-config-reloader

USER nobody

ENTRYPOINT ["/bin/prometheus-config-reloader"]
