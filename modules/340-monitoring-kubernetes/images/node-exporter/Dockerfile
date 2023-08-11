ARG BASE_DISTROLESS
ARG BASE_GOLANG_19_ALPINE

FROM $BASE_GOLANG_19_ALPINE as artifact

ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}

RUN apk add --no-cache git
RUN git clone --depth 1 --branch v0.18.1 ${SOURCE_REPO}/prometheus/node_exporter.git /node_exporter
WORKDIR /node_exporter/
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o node_exporter node_exporter.go && \
    chown -R 64535:64535 /node_exporter/ && \
    chmod 0700 /node_exporter/node_exporter

FROM $BASE_DISTROLESS
COPY --from=artifact /node_exporter/node_exporter /bin

EXPOSE      9100
USER        64535

ENTRYPOINT  [ "/bin/node_exporter" ]
