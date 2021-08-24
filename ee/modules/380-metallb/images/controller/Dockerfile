# Based on https://github.com/metallb/metallb/blob/v0.10.2/speaker/Dockerfile
ARG BASE_ALPINE
FROM metallb/controller:v0.10.2@sha256:70f08345cd5f8c62f7857ec724a4313e49357925ccb4a7d5ea6cf306b350650b as artifact

FROM $BASE_ALPINE
COPY --from=artifact /controller /
ENTRYPOINT ["/controller"]
