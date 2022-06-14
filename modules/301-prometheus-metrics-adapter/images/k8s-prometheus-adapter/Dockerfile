ARG BASE_ALPINE
FROM registry.k8s.io/prometheus-adapter/prometheus-adapter:v0.9.1@sha256:d025d1a109234c28b4a97f5d35d759943124be8885a5bce22a91363025304e9d as artifact

FROM $BASE_ALPINE
COPY --from=artifact /adapter /adapter
ENTRYPOINT [ "/adapter" ]

