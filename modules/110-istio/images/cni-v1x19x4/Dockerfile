# Based on https://github.com/istio/istio/blob/1.19.4/cni/deployments/kubernetes/Dockerfile.install-cni
ARG BASE_UBUNTU
FROM docker.io/istio/install-cni:1.19.4@sha256:f27f76993f46db2827b2381daf66d9354ac3a34d4040eecc1375819c8eb085ad as artifact

FROM $BASE_UBUNTU

# install operator binary
COPY --from=artifact /opt/cni/bin/istio-cni /opt/cni/bin/istio-cni
COPY --from=artifact /usr/local/bin/install-cni /usr/local/bin/install-cni

ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/cni/bin
WORKDIR /opt/cni/bin
USER 1337:1337

ENTRYPOINT ["/usr/local/bin/install-cni"]
