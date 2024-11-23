ARG BASE_DISTROLESS
ARG BASE_GOLANG_19_ALPINE

# Based on https://github.com/nabokihms/events_exporter/blob/main/Dockerfile
FROM $BASE_GOLANG_19_ALPINE as artifact
RUN apk add --no-cache --update alpine-sdk bash openssh-client

WORKDIR /usr/local/src/events_exporter

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT=""

ENV GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT}
ARG GOPROXY

ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}

ARG GOPROXY
ENV GOPROXY=${GOPROXY}

ENV GOARCH=amd64
RUN mkdir -p ~/.ssh && echo "StrictHostKeyChecking accept-new" > ~/.ssh/config
RUN apk add --no-cache make git
RUN --mount=type=ssh git clone --branch v0.0.4 --depth 1 ${SOURCE_REPO}/nabokihms/events_exporter.git . && \
    make build && \
    chown -R 64535:64535 /usr/local/src/ && \
    chmod 0700 /usr/local/src/events_exporter/bin/events_exporter

FROM $BASE_DISTROLESS
COPY --from=artifact /usr/local/src/events_exporter/bin/events_exporter /usr/local/bin/events_exporter

ENTRYPOINT ["events_exporter"]
