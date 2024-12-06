# Based on https://github.com/istio/istio/blob/1.21.6/pilot/docker/Dockerfile.pilot
ARG BASE_ALT

FROM docker.io/istio/pilot:1.21.6@sha256:c2aec3195dc7f50150c23d7dd891e6a15909f75db6fea9ddc954b63233c4406c as artifact

FROM $BASE_ALT
RUN apt-get update && \
    apt-get -y install ca-certificates && \
    apt-get clean && \
    apt-get autoremove -y && \
    rm -rf /var/lib/apt/lists/*

COPY --from=artifact /usr/local/bin/pilot-discovery /usr/local/bin/
COPY --from=artifact /var/lib/istio/envoy/envoy_bootstrap_tmpl.json /var/lib/istio/envoy/envoy_bootstrap_tmpl.json
COPY --from=artifact /var/lib/istio/envoy/gcp_envoy_bootstrap_tmpl.json /var/lib/istio/envoy/gcp_envoy_bootstrap_tmpl.json

USER 1337:1337

ENTRYPOINT ["/usr/local/bin/pilot-discovery"]
