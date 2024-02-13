# Based on https://github.com/istio/istio/blob/1.19.4/cni/deployments/kubernetes/Dockerfile.install-cni
ARG BASE_UBUNTU
FROM docker.io/istio/install-cni:1.12.6@sha256:53a0b30defa86096ffca33432fa9f2d7aa26fce8a18644e6d41da79bb0126074 as artifact

FROM $BASE_UBUNTU

# install operator binary
COPY --from=artifact /opt/cni/bin/istio-cni /opt/cni/bin/istio-cni
COPY --from=artifact /usr/local/bin/install-cni /usr/local/bin/install-cni

ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/cni/bin
WORKDIR /opt/cni/bin
USER 1337:1337

ENTRYPOINT ["/usr/local/bin/install-cni"]
