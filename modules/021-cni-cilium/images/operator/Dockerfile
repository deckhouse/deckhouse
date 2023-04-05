# Based on https://github.com/cilium/cilium/blob/v1.12.8/images/operator/Dockerfile
ARG BASE_ALPINE
# https://github.com/cilium/cilium/releases/tag/v1.12.8
FROM quay.io/cilium/operator:v1.12.8@sha256:1d3f32b112034dc0a7b83cde55850f00cf3adca9ae7f51aff42f2f8228998c8b as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/bin/cilium-operator /usr/bin/cilium-operator

RUN apk add --no-cache ca-certificates

USER nobody
ENTRYPOINT ["/usr/bin/cilium-operator"]
