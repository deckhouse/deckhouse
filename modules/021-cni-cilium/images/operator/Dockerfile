# Based on https://github.com/cilium/cilium/blob/956c4d670fd75eb9f2a5a44406bc02aeab820cd7/images/operator/Dockerfile
ARG BASE_ALPINE
# Source: https://github.com/cilium/cilium/releases/tag/v1.12.4
FROM quay.io/cilium/operator:v1.12.4@sha256:95c9a3be33ec87fe404cc876719f6a06a27658464dab3efbedd9974969cab21c as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/bin/cilium-operator /usr/bin/cilium-operator

RUN apk add --no-cache ca-certificates

USER nobody
ENTRYPOINT ["/usr/bin/cilium-operator"]
