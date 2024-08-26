# Based on https://github.com/istio/istio/blob/1.16.2/cni/deployments/kubernetes/Dockerfile.install-cni
ARG BASE_ALPINE
FROM docker.io/istio/install-cni:1.16.2@sha256:c5814f65a9317ceaf41108350964c089db8cbeec76f66994509b5e371ce5ef72 as artifact

FROM $BASE_ALPINE

# install operator binary
COPY --from=artifact /opt/cni/bin/istio-cni /opt/cni/bin/istio-cni
COPY --from=artifact /usr/local/bin/install-cni /usr/local/bin/install-cni

ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/cni/bin
WORKDIR /opt/cni/bin

ENTRYPOINT ["/usr/local/bin/install-cni"]
