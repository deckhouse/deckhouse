# Based on https://github.com/istio/istio/blob/1.21.6/docker/Dockerfile.base
#      and https://github.com/istio/istio/blob/1.21.6/pilot/docker/Dockerfile.proxyv2
ARG BASE_ALT
FROM docker.io/istio/proxyv2:1.21.6@sha256:a0afbbc8488060f8059e7fa04576edde01915d40576132dae83479b3abe5ce15 as artifact

FROM $BASE_ALT
WORKDIR /

# https://hub.docker.com/layers/istio/proxyv2/1.21.6/images/sha256-1d1cbdc7d940e09ce3d636897db74c77d4fff4c91f3c97fad96ed0bbb4c7eee4?context=explore
# from image layer 17
ARG proxy_version=istio-proxy:78bd2d9b284978e170a49cd13decd5f952544489
ARG istio_version=1.21.6
ARG SIDECAR=envoy

RUN apt-get update && \
  apt-get install -y \
  ca-certificates \
  curl \
  iptables \
  iproute2 \
  iputils \
  knot-utils \
  netcat \
  tcpdump \
  conntrack-tools \
  whois \
  net-tools \
  lsof \
  sudo \
  && update-ca-trust \
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
