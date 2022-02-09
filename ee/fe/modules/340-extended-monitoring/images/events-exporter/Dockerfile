ARG BASE_ALPINE
ARG BASE_GOLANG_16_ALPINE

# Based on https://github.com/nabokihms/events_exporter/blob/main/Dockerfile
FROM $BASE_GOLANG_16_ALPINE as artifact
RUN apk add --no-cache --update alpine-sdk bash

WORKDIR /usr/local/src/events_exporter

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT=""

ENV GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT}
ARG GOPROXY

ENV GOARCH=amd64
RUN apk add --no-cache make git && \
    git clone --branch v0.0.2 --depth 1 https://github.com/nabokihms/events_exporter.git . && \
    make build


FROM $BASE_ALPINE
RUN apk add --no-cache --update ca-certificates
COPY --from=artifact /usr/local/src/events_exporter/bin/events_exporter /usr/local/bin/events_exporter

# nobody
USER 1001:1001

ENTRYPOINT ["events_exporter"]
