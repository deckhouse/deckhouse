# Based on https://github.com/istio/istio/blob/1.21.6/cni/deployments/kubernetes/Dockerfile.install-cni
ARG BASE_ALT
FROM docker.io/istio/install-cni:1.21.6@sha256:6c85a016e612d7a095ed32546dbe7f973672b31cc1930c26a747232d38ff372a as artifact

FROM $BASE_ALT

# install operator binary
COPY --from=artifact /opt/cni/bin/istio-cni /opt/cni/bin/istio-cni
COPY --from=artifact /usr/local/bin/install-cni /usr/local/bin/install-cni

ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/cni/bin
WORKDIR /opt/cni/bin

ENTRYPOINT ["/usr/local/bin/install-cni"]
