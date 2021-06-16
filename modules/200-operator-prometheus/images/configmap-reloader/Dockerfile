# Based on https://github.com/jimmidyson/configmap-reload/blob/master/Dockerfile
ARG BASE_ALPINE
FROM quay.io/coreos/configmap-reload:v0.0.1@sha256:e2fd60ff0ae4500a75b80ebaa30e0e7deba9ad107833e8ca53f0047c42c5a057 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /configmap-reload /

ENTRYPOINT ["/configmap-reload"]
