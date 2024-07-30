# Based on https://github.com/istio/istio/blob/1.19.7/operator/docker/Dockerfile.operator
ARG BASE_UBUNTU
FROM docker.io/istio/operator:1.19.7@sha256:f7f371476a2224fe55ff4e7c8b31773e2c048da255bb634075666f88fa7cb990 as artifact

FROM $BASE_UBUNTU

# install operator binary
COPY --from=artifact /usr/local/bin/operator /usr/local/bin/

# add operator manifests
COPY --from=artifact /var/lib/istio/manifests/ /var/lib/istio/manifests/

USER 1337:1337

ENTRYPOINT ["/usr/local/bin/operator"]
