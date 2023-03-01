# Based on https://github.com/istio/istio/blob/1.16.2/docker/Dockerfile.base
#      and https://github.com/istio/istio/blob/1.16.2/pilot/docker/Dockerfile.proxyv2
ARG BASE_UBUNTU
FROM docker.io/istio/proxyv2:1.16.2@sha256:e3122850ded1a844bd7d74afafbd3e388221c9e916c4e0ac47af8b644b939871 as artifact

FROM $BASE_UBUNTU
WORKDIR /

# https://hub.docker.com/layers/istio/proxyv2/1.16.2/images/sha256-d50597afa2ac9a9ddad16d1f4065c83326d4b21526e1c4fc997e7f9f21d65c53?context=explore
# from image layer 14
ARG proxy_version=istio-proxy:fa88959eb040f004889985399669f7a824a7e18e
ARG istio_version=1.16.2
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

COPY --from=artifact /etc/istio/extensions/stats-filter.wasm /etc/istio/extensions/stats-filter.wasm
COPY --from=artifact /etc/istio/extensions/stats-filter.compiled.wasm /etc/istio/extensions/stats-filter.compiled.wasm
COPY --from=artifact /etc/istio/extensions/metadata-exchange-filter.wasm /etc/istio/extensions/metadata-exchange-filter.wasm
COPY --from=artifact /etc/istio/extensions/metadata-exchange-filter.compiled.wasm /etc/istio/extensions/metadata-exchange-filter.compiled.wasm

# The pilot-agent will bootstrap Envoy.
ENTRYPOINT ["/usr/local/bin/pilot-agent"]
