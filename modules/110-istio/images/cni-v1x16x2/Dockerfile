# Based on https://github.com/istio/istio/blob/1.16.2/cni/deployments/kubernetes/Dockerfile.install-cni
ARG BASE_ALPINE
FROM docker.io/istio/install-cni:1.16.2@sha256:14255494cf4c78740913c702aa0768fad4b433e82d10cdfc264ef5f30cf041ec as artifact

FROM $BASE_ALPINE

# install operator binary
COPY --from=artifact /opt/cni/bin/istio-cni /opt/cni/bin/istio-cni
COPY --from=artifact /usr/local/bin/install-cni /usr/local/bin/install-cni

ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/cni/bin
WORKDIR /opt/cni/bin

ENTRYPOINT ["/usr/local/bin/install-cni"]
