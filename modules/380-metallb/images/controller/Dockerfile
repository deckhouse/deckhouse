# Based on https://github.com/metallb/metallb/blob/v0.8.2/speaker/Dockerfile
ARG BASE_ALPINE
FROM metallb/controller:v0.8.2@sha256:5c050e59074e152711737d2bb9ede96dff67016c80cf25cdf5fc46109718a583 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /controller /
ENTRYPOINT ["/controller"]
