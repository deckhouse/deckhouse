# Based on https://github.com/istio/istio/blob/1.13.3/pilot/docker/Dockerfile.pilot
ARG BASE_DEBIAN

FROM docker.io/istio/pilot:1.13.3@sha256:dec8156bed76ea584b5285e8d427ca6421d9f79949472792bc3132991dc7ee57 as artifact

FROM $BASE_DEBIAN
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
