# Based on https://github.com/istio/istio/blob/1.13.9/pilot/docker/Dockerfile.pilot
ARG BASE_UBUNTU

FROM docker.io/istio/pilot:1.13.9@sha256:2b514a319d0ec59223c8fd5ff86eb0b6764a4fda230f0ac707bdf7a956c74058 as artifact

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
