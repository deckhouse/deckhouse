# Based on https://github.com/cilium/cilium/blob/956c4d670fd75eb9f2a5a44406bc02aeab820cd7/images/hubble-relay/Dockerfile
ARG BASE_ALPINE
FROM quay.io/cilium/hubble-relay@sha256:ebe180f78578d45565984620b672cd942e7a654fbc5b4c5a0c2139d15e6ac4dc as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/bin/hubble-relay /usr/bin/hubble-relay

RUN apk add --no-cache ca-certificates

USER nobody
ENTRYPOINT ["/usr/bin/hubble-relay"]
