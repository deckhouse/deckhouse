# Based on https://github.com/istio/istio/blob/1.21.6/operator/docker/Dockerfile.operator
ARG BASE_UBUNTU
FROM docker.io/istio/operator:1.21.6@sha256:436ac8dd5913cb82bdad5f3975a853fa76e73d0cd92d05f6f7fd78c0a32f1332 as artifact

FROM $BASE_UBUNTU

# install operator binary
COPY --from=artifact /usr/local/bin/operator /usr/local/bin/

# add operator manifests
COPY --from=artifact /var/lib/istio/manifests/ /var/lib/istio/manifests/

USER 1337:1337

ENTRYPOINT ["/usr/local/bin/operator"]
