# Based on https://github.com/istio/istio/blob/1.10.0/pilot/docker/Dockerfile.pilot
ARG BASE_DEBIAN

FROM docker.io/istio/pilot:1.10.0@sha256:294ca55bd1cc6cd629fd204e4edba42b2f05c2948d7e802d78e81423aef2ff05 as artifact

FROM $BASE_DEBIAN

COPY --from=artifact /usr/local/bin/pilot-discovery /usr/local/bin/

USER 1337:1337

ENTRYPOINT ["/usr/local/bin/pilot-discovery"]
