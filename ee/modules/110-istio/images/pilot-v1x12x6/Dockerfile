# Based on https://github.com/istio/istio/blob/1.12.6/pilot/docker/Dockerfile.pilot
ARG BASE_UBUNTU

FROM docker.io/istio/pilot:1.12.6@sha256:a930b1a37df46c70a4d715fe11999a7310303dd1d49092616c6571b5f13a7ce6 as artifact

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
