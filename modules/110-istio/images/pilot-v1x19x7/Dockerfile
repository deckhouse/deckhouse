# Based on https://github.com/istio/istio/blob/1.19.7/pilot/docker/Dockerfile.pilot
ARG BASE_UBUNTU

FROM docker.io/istio/pilot:1.19.7@sha256:c5feddb54bdc58ddb8b5ed462547b4a673f2b9c5163fe42cec0c0f0115c4c360 as artifact

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
