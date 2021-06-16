# Based on https://github.com/istio/istio/blob/1.10.0/pilot/docker/Dockerfile.proxyv2
ARG BASE_DEBIAN
FROM docker.io/istio/proxyv2:1.10.0@sha256:88c6c693e67a0f2492191a0e7d8020ddc85603bfc704f252655cb9eb5eeb3f58 as artifact

FROM $BASE_DEBIAN
WORKDIR /

ARG proxy_version
ARG istio_version
ARG SIDECAR=envoy

# Copy Envoy bootstrap templates used by pilot-agent
COPY --from=artifact /var/lib/istio/envoy/envoy_bootstrap_tmpl.json /var/lib/istio/envoy/envoy_bootstrap_tmpl.json
COPY --from=artifact /var/lib/istio/envoy/gcp_envoy_bootstrap_tmpl.json /var/lib/istio/envoy/gcp_envoy_bootstrap_tmpl.json
# Install Envoy.
COPY --from=artifact /usr/local/bin/envoy /usr/local/bin/envoy
COPY --from=artifact /usr/local/bin/pilot-agent /usr/local/bin/pilot-agent
COPY --from=artifact /etc/istio/extensions/stats-filter.wasm /etc/istio/extensions/stats-filter.wasm
COPY --from=artifact /etc/istio/extensions/stats-filter.compiled.wasm /etc/istio/extensions/stats-filter.compiled.wasm
COPY --from=artifact /etc/istio/extensions/metadata-exchange-filter.wasm /etc/istio/extensions/metadata-exchange-filter.wasm
COPY --from=artifact /etc/istio/extensions/metadata-exchange-filter.compiled.wasm /etc/istio/extensions/metadata-exchange-filter.compiled.wasm

# Environment variable indicating the exact proxy sha - for debugging or version-specific configs
ENV ISTIO_META_ISTIO_PROXY_SHA istio-proxy:436f365a8007cd8a13a9f1321e7cce94bcc8883e
# Environment variable indicating the exact build, for debugging
ENV ISTIO_META_ISTIO_VERSION 1.10.0

# The pilot-agent will bootstrap Envoy.
ENTRYPOINT ["/usr/local/bin/pilot-agent"]
