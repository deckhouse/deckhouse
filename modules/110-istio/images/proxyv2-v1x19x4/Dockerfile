# Based on https://github.com/istio/istio/blob/1.19.4/docker/Dockerfile.base
#      and https://github.com/istio/istio/blob/1.19.4/pilot/docker/Dockerfile.proxyv2
ARG BASE_UBUNTU
FROM docker.io/istio/proxyv2:1.19.4@sha256:6f72d576ce64616342df2ed7246eb33a9c59f01d63741b7ac0791cbf719c7624 as artifact

FROM $BASE_UBUNTU
WORKDIR /

# https://hub.docker.com/layers/istio/proxyv2/1.19.4/images/sha256-6f72d576ce64616342df2ed7246eb33a9c59f01d63741b7ac0791cbf719c7624?context=explore
# from image layer 17
ARG proxy_version=istio-proxy:f9707e29aa0a36b1430b373d95e6c9abb5deca75
ARG istio_version=1.19.4
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
