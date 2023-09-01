ARG BASE_GOLANG_19_ALPINE
ARG BASE_DISTROLESS
ARG BASE_ALPINE

FROM $BASE_GOLANG_19_ALPINE as artifact

ARG GOPROXY
ENV GOPROXY=${GOPROXY}

WORKDIR /src/
COPY exporter/ /src/
RUN apk add --no-cache git && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o loop main.go && \
    chown -R 64535:64535 /src/ && \
    chmod 0700 /src/loop && \
    mkdir -p /var/run/node-exporter-textfile && \
    chown -R 64535:64535 /var/run/node-exporter-textfile && \
    chmod 0600 /var/run/node-exporter-textfile

FROM $BASE_ALPINE

COPY --from=artifact /src/loop /
COPY --from=artifact /var/run/node-exporter-textfile /var/run/node-exporter-textfile

ENTRYPOINT ["while :; do sleep 1; done;"]
