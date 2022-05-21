# Based on https://github.com/istio/istio/blob/1.13.3/operator/docker/Dockerfile.operator
ARG BASE_DEBIAN
FROM docker.io/istio/operator:1.13.3@sha256:fbf1c3ad009106632951ba2a5ba978be789f627063ff3df4cebadb8683ea4650 as artifact

FROM $BASE_DEBIAN

# install operator binary
COPY --from=artifact /usr/local/bin/operator /usr/local/bin/

# add operator manifests
COPY --from=artifact /var/lib/istio/manifests/ /var/lib/istio/manifests/

USER 1337:1337

ENTRYPOINT ["/usr/local/bin/operator"]
