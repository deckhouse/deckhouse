# Based on https://github.com/istio/istio/blob/1.10.1/operator/docker/Dockerfile.operator
ARG BASE_DEBIAN
FROM docker.io/istio/operator:1.10.1@sha256:29efe029daa11cf9925fa4ef46486a33c72c8819126bf8eae7cb03ade49cad7c as artifact

FROM $BASE_DEBIAN

# install operator binary
COPY --from=artifact /usr/local/bin/operator /usr/local/bin/

# add operator manifests
COPY --from=artifact /var/lib/istio/manifests/ /var/lib/istio/manifests/

USER 1337:1337

ENTRYPOINT ["/usr/local/bin/operator"]
