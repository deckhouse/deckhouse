# Based on https://github.com/istio/istio/blob/1.13.7/pilot/docker/Dockerfile.pilot
ARG BASE_UBUNTU

FROM docker.io/istio/pilot:1.13.7@sha256:3702d411ded12867cdc3966e84ce4d74c9c091bcd98b0d7e8a2c7946182797f9 as artifact

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
