# Based on https://github.com/metallb/metallb/blob/v0.10.2/speaker/Dockerfile
ARG BASE_ALPINE
FROM quay.io/metallb/controller:v0.11.0@sha256:f269f7db7b445eaaf668e021c961bfb877b29a45999c0a22e787400607e51ae8 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /controller /
ENTRYPOINT ["/controller"]
