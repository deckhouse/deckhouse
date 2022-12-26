# Based on https://github.com/cilium/cilium/blob/v1.12.5/images/operator/Dockerfile
ARG BASE_ALPINE
# Image source: https://github.com/cilium/cilium/releases/tag/v1.12.4
FROM quay.io/cilium/operator:v1.12.5@sha256:a6d24a006a6b92967ac90786b49bc1ac26e5477cf028cd1186efcfc2466484db as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/bin/cilium-operator /usr/bin/cilium-operator

RUN apk add --no-cache ca-certificates

USER nobody
ENTRYPOINT ["/usr/bin/cilium-operator"]
