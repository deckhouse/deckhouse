# Based on https://github.com/istio/istio/blob/1.10.1/pilot/docker/Dockerfile.pilot
ARG BASE_DEBIAN

FROM docker.io/istio/pilot:1.10.1@sha256:fd158a373928927774d1129adc894507f7e452d401de785f4ab2a675a0d43932 as artifact

FROM $BASE_DEBIAN

COPY --from=artifact /usr/local/bin/pilot-discovery /usr/local/bin/

USER 1337:1337

ENTRYPOINT ["/usr/local/bin/pilot-discovery"]
