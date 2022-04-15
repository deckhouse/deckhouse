# Based on https://github.com/cilium/cilium/blob/956c4d670fd75eb9f2a5a44406bc02aeab820cd7/images/operator/Dockerfile
ARG BASE_ALPINE
FROM quay.io/cilium/operator@sha256:1b98424b99f5f09bc1d97ac7b4a099fe1fb868078feb23d500c9395a73fe0a54 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/bin/cilium-operator /usr/bin/cilium-operator

RUN apk add --no-cache ca-certificates

USER nobody
ENTRYPOINT ["/usr/bin/cilium-operator"]
