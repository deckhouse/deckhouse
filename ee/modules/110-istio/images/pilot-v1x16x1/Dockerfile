# Based on https://github.com/istio/istio/blob/1.16.1/pilot/docker/Dockerfile.pilot
ARG BASE_UBUNTU

FROM docker.io/istio/pilot:1.16.1@sha256:0f0c14b1a6592f994b091433042b83c10adfac185e7cd1abea033b3cb9508ddd as artifact

FROM $BASE_UBUNTU
RUN apt-get update && \
    apt-get -y --no-install-recommends install ca-certificates && \
    apt-get clean && \
    apt-get autoremove -y && \
    rm -rf /var/lib/apt/lists/*

COPY --from=artifact /usr/local/bin/pilot-discovery /usr/local/bin/
COPY --from=artifact /var/lib/istio/envoy/envoy_bootstrap_tmpl.json /var/lib/istio/envoy/envoy_bootstrap_tmpl.json
COPY --from=artifact /var/lib/istio/envoy/gcp_envoy_bootstrap_tmpl.json /var/lib/istio/envoy/gcp_envoy_bootstrap_tmpl.json

USER 1337:1337

ENTRYPOINT ["/usr/local/bin/pilot-discovery"]
