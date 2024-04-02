# Based on https://github.com/istio/istio/blob/1.19.7/docker/Dockerfile.base
#      and https://github.com/istio/istio/blob/1.19.7/pilot/docker/Dockerfile.proxyv2
ARG BASE_UBUNTU
FROM docker.io/istio/proxyv2:1.19.7@sha256:680eac0faff1835f994ba9047750d7cfe989e868324a933a38d6a6cb80779b33 as artifact

FROM $BASE_UBUNTU
WORKDIR /

# https://hub.docker.com/layers/istio/proxyv2/1.19.7/images/sha256-680eac0faff1835f994ba9047750d7cfe989e868324a933a38d6a6cb80779b33?context=explore
# from image layer 17
ARG proxy_version=istio-proxy:af5e0ef2c1473f0f4e61f78adf81c85ff6389f87
ARG istio_version=1.19.7
ARG SIDECAR=envoy

RUN apt-get update && \
  apt-get install --no-install-recommends -y \
  ca-certificates \
  curl \
  iptables \
  iproute2 \
  iputils-ping \
  knot-dnsutils \
  netcat \
  tcpdump \
  conntrack \
  bsdmainutils \
  net-tools \
  lsof \
  sudo \
  && update-ca-certificates \
  && apt-get upgrade -y \
  && apt-get clean \
  && rm -rf  /var/log/*log /var/lib/apt/lists/* /var/log/apt/* /var/lib/dpkg/*-old /var/cache/debconf/*-old \
  && update-alternatives --set iptables /usr/sbin/iptables-legacy \
  && update-alternatives --set ip6tables /usr/sbin/ip6tables-legacy


RUN useradd -m --uid 1337 istio-proxy && echo "istio-proxy ALL=NOPASSWD: ALL" >> /etc/sudoers

# Copy Envoy bootstrap templates used by pilot-agent
COPY --from=artifact /var/lib/istio/envoy/envoy_bootstrap_tmpl.json /var/lib/istio/envoy/envoy_bootstrap_tmpl.json
COPY --from=artifact /var/lib/istio/envoy/gcp_envoy_bootstrap_tmpl.json /var/lib/istio/envoy/gcp_envoy_bootstrap_tmpl.json

# Install Envoy.
COPY --from=artifact /usr/local/bin/$SIDECAR /usr/local/bin/$SIDECAR

# Environment variable indicating the exact proxy sha - for debugging or version-specific configs
ENV ISTIO_META_ISTIO_PROXY_SHA $proxy_version
# Environment variable indicating the exact build, for debugging
ENV ISTIO_META_ISTIO_VERSION $istio_version

COPY --from=artifact /usr/local/bin/pilot-agent /usr/local/bin/pilot-agent

# The pilot-agent will bootstrap Envoy.
ENTRYPOINT ["/usr/local/bin/pilot-agent"]
