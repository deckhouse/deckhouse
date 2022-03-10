# Based on https://github.com/cilium/cilium/blob/956c4d670fd75eb9f2a5a44406bc02aeab820cd7/images/hubble-relay/Dockerfile
ARG BASE_ALPINE
FROM quay.io/cilium/hubble-relay@sha256:7dfa201342d2c7bbe9c46a9b3511e140f2128d40e5a36aa838d5e21154489794 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/bin/hubble-relay /usr/bin/hubble-relay

RUN apk add --no-cache ca-certificates

USER nobody
ENTRYPOINT ["/usr/bin/hubble-relay"]
