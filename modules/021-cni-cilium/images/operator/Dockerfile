# Based on https://github.com/cilium/cilium/blob/956c4d670fd75eb9f2a5a44406bc02aeab820cd7/images/operator/Dockerfile
ARG BASE_ALPINE
# https://github.com/cilium/cilium/releases/tag/v1.11.12
FROM quay.io/cilium/operator:v1.11.13@sha256:859ff723b1b6c03f7f0126383e619fcff9fb78648eca6f4c6a4412989047f9da as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/bin/cilium-operator /usr/bin/cilium-operator

RUN apk add --no-cache ca-certificates

USER nobody
ENTRYPOINT ["/usr/bin/cilium-operator"]
