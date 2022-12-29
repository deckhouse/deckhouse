# Based on https://github.com/cilium/cilium/blob/956c4d670fd75eb9f2a5a44406bc02aeab820cd7/images/operator/Dockerfile
ARG BASE_ALPINE
# https://github.com/cilium/cilium/releases/tag/v1.11.12
FROM quay.io/cilium/operator:v1.11.12@sha256:1d9c408c48d88faea6766ce012829560cd905ac3a9f40397a571dd580c6e6381 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/bin/cilium-operator /usr/bin/cilium-operator

RUN apk add --no-cache ca-certificates

USER nobody
ENTRYPOINT ["/usr/bin/cilium-operator"]
