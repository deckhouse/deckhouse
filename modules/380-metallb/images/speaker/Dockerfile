# Based on https://github.com/metallb/metallb/blob/v0.8.2/speaker/Dockerfile
ARG BASE_ALPINE
FROM metallb/speaker:v0.8.2@sha256:f1941498a28cdb332429e25d18233683da6949ecfc4f6dacf12b1416d7d38263 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /speaker /
ENTRYPOINT ["/speaker"]
