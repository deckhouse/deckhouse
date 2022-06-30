# Based on https://github.com/istio/istio/blob/1.13.3/pilot/docker/Dockerfile.proxyv2
ARG BASE_DEBIAN
FROM docker.io/istio/proxyv2:1.13.3@sha256:e8986efce46a7e1fcaf837134f453ea2b5e0750a464d0f2405502f8ddf0e2cd2 as artifact

FROM $BASE_DEBIAN
WORKDIR /

# https://hub.docker.com/layers/proxyv2/istio/proxyv2/1.13.3/images/sha256-e8986efce46a7e1fcaf837134f453ea2b5e0750a464d0f2405502f8ddf0e2cd2?context=explore
# line 14
ARG proxy_version=istio-proxy:8df7c693c8d6d0679dd72e0130f34a0ee91c210b
ARG istio_version=1.13.3
ARG SIDECAR=envoy

RUN apt-get update && \
   apt-get install --no-install-recommends -y \
   ca-certificates curl iptables iproute2 iputils-ping \
   knot-dnsutils netcat tcpdump conntrack bsdmainutils net-tools lsof sudo && \
   update-ca-certificates && \
   apt-get upgrade -y && \
   apt-get clean && \
   rm -rf /var/log/*log /var/lib/apt/lists/* /var/log/apt/* /var/lib/dpkg/*-old /var/cache/debconf/*-old

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
