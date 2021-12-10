# Based on https://github.com/cilium/cilium/blob/956c4d670fd75eb9f2a5a44406bc02aeab820cd7/images/hubble-relay/Dockerfile
ARG BASE_ALPINE
FROM quay.io/cilium/hubble-relay@sha256:d2ebf2da18e2f945d019d46a178c0686776b360c289a0bb6b1f4fd7ca78587d2 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/bin/hubble-relay /usr/bin/hubble-relay

RUN apk add --no-cache ca-certificates

USER nobody
ENTRYPOINT ["/usr/bin/hubble-relay"]
