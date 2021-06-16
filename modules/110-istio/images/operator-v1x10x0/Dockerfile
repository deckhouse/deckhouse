# Based on https://github.com/istio/istio/blob/1.10.0/operator/docker/Dockerfile.operator
ARG BASE_DEBIAN
FROM docker.io/istio/operator:1.10.0@sha256:30991566a6b8616672bfdbb3e33d03ab3a3d0d7253d348c9814e68eb47b19455 as artifact

FROM $BASE_DEBIAN

# install operator binary
COPY --from=artifact /usr/local/bin/operator /usr/local/bin/

# add operator manifests
COPY --from=artifact /var/lib/istio/manifests/ /var/lib/istio/manifests/

USER 1337:1337

ENTRYPOINT ["/usr/local/bin/operator"]
