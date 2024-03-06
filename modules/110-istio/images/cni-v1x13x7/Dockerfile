# Based on https://github.com/istio/istio/blob/1.13.7/cni/deployments/kubernetes/Dockerfile.install-cni
ARG BASE_ALPINE
FROM docker.io/istio/install-cni:1.13.7@sha256:dd276eb1691d6f80f35b33f34cd143130d6aa9f877fcb2c4aad1c21c7633dd54 as artifact

FROM $BASE_ALPINE

# install operator binary
COPY --from=artifact /opt/cni/bin/istio-cni /opt/cni/bin/istio-cni
COPY --from=artifact /usr/local/bin/install-cni /usr/local/bin/install-cni

ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/cni/bin
WORKDIR /opt/cni/bin

ENTRYPOINT ["/usr/local/bin/install-cni"]
