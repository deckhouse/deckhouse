# Based on https://github.com/cilium/cilium/blob/956c4d670fd75eb9f2a5a44406bc02aeab820cd7/images/operator/Dockerfile
ARG BASE_ALPINE
# https://github.com/cilium/cilium/releases/tag/v1.11.14
FROM quay.io/cilium/operator:v1.11.14@sha256:5e3da0ece8520c95f252b904de6fd670bba72ef7b0c8eba2cfba992dcc18cb97 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/bin/cilium-operator /usr/bin/cilium-operator

RUN apk add --no-cache ca-certificates

USER nobody
ENTRYPOINT ["/usr/bin/cilium-operator"]
