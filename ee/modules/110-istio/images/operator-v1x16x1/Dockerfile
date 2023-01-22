# Based on https://github.com/istio/istio/blob/1.16.1/operator/docker/Dockerfile.operator
ARG BASE_UBUNTU
FROM docker.io/istio/operator:1.16.1@sha256:9763f0d5278699648fb810db0d56332efff02c45b32bb7586ac67bb9e75e7151 as artifact

FROM $BASE_UBUNTU

# install operator binary
COPY --from=artifact /usr/local/bin/operator /usr/local/bin/

# add operator manifests
COPY --from=artifact /var/lib/istio/manifests/ /var/lib/istio/manifests/

USER 1337:1337

ENTRYPOINT ["/usr/local/bin/operator"]
