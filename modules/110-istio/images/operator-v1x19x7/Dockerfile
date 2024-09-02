# Based on https://github.com/istio/istio/blob/1.19.7/operator/docker/Dockerfile.operator
ARG BASE_UBUNTU
FROM docker.io/istio/operator:1.19.7@sha256:5b3c7f9d168ba9a44a5c4532c5282ff590effc66b3f115d713aab3b0b68d22e3 as artifact

FROM $BASE_UBUNTU

# install operator binary
COPY --from=artifact /usr/local/bin/operator /usr/local/bin/

# add operator manifests
COPY --from=artifact /var/lib/istio/manifests/ /var/lib/istio/manifests/

USER 1337:1337

ENTRYPOINT ["/usr/local/bin/operator"]
