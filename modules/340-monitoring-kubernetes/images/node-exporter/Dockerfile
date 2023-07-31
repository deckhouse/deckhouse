ARG BASE_DISTROLESS
ARG BASE_GOLANG_19_ALPINE

FROM $BASE_GOLANG_19_ALPINE as artifact

RUN apk add --no-cache git
RUN git clone --depth 1 --branch v0.18.1 https://github.com/prometheus/node_exporter.git /node_exporter
WORKDIR /node_exporter/
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o node_exporter node_exporter.go

FROM $BASE_DISTROLESS
COPY --from=artifact /node_exporter/node_exporter /bin

EXPOSE      9100
USER        nobody

ENTRYPOINT  [ "/bin/node_exporter" ]
