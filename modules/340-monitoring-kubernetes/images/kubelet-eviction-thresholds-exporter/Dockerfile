ARG BASE_GOLANG_19_ALPINE
ARG BASE_DISTROLESS

FROM $BASE_GOLANG_19_ALPINE as artifact

WORKDIR /src/
COPY exporter/ /src/
RUN apk add --no-cache git && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o loop main.go && \
    mkdir -p /var/run/node-exporter-textfile/ && \
    chown -R 64535:64535 /var/run/node-exporter-textfile/ && \
    chown -R 64535:64535 /src/ && \
    chmod 0600 -R /var/run/node-exporter-textfile && \
    chmod 0700 /src/loop

FROM $BASE_DISTROLESS

COPY --from=artifact /src/loop /
COPY --from=artifact /var/run/node-exporter-textfile/ /var/run/node-exporter-textfile/
USER 64535
ENTRYPOINT ["/loop"]
