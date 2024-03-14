# Based on https://github.com/istio/istio/blob/1.19.7/cni/deployments/kubernetes/Dockerfile.install-cni
ARG BASE_ALPINE
FROM docker.io/istio/install-cni:1.19.7@sha256:94b0b7728cfaeb0595afbcd69aad05f8c4fbda2dbfdbeddb25d12934c0f673b1 as artifact

FROM $BASE_ALPINE

# install operator binary
COPY --from=artifact /opt/cni/bin/istio-cni /opt/cni/bin/istio-cni
COPY --from=artifact /usr/local/bin/install-cni /usr/local/bin/install-cni

ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/cni/bin
WORKDIR /opt/cni/bin

ENTRYPOINT ["/usr/local/bin/install-cni"]
