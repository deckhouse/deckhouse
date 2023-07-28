ARG BASE_DISTROLESS
ARG BASE_GOLANG_19_ALPINE

# Based on https://github.com/nabokihms/events_exporter/blob/main/Dockerfile
FROM $BASE_GOLANG_19_ALPINE as artifact
RUN apk add --no-cache --update alpine-sdk bash

WORKDIR /usr/local/src/events_exporter

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT=""

ENV GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT}
ARG GOPROXY

ARG SOURCE_REPO

ENV GOARCH=amd64
RUN apk add --no-cache make git && \
    git clone --branch v0.0.4 --depth 1 ${SOURCE_REPO-https://github.com}/nabokihms/events_exporter.git . && \
    make build


FROM $BASE_DISTROLESS
COPY --from=artifact /usr/local/src/events_exporter/bin/events_exporter /usr/local/bin/events_exporter

# nobody
USER 1001:1001

ENTRYPOINT ["events_exporter"]
