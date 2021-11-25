# Based on https://github.com/metallb/metallb/blob/v0.10.2/speaker/Dockerfile
ARG BASE_ALPINE
FROM metallb/controller:v0.11.0@sha256:55b4ff4dbbce4cd87e30e5c8214dbee49d73cf8a2fef05fa2d16d8af58beea83 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /controller /
ENTRYPOINT ["/controller"]
