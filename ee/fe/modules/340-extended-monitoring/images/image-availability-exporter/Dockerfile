ARG BASE_ALPINE
ARG BASE_GOLANG_17_ALPINE

# Based on https://github.com/deckhouse/k8s-image-availability-exporter/blob/master/Dockerfile
FROM $BASE_GOLANG_17_ALPINE as artifact
WORKDIR /src
ENV GOARCH=amd64
RUN apt add --no-cache git && \
    git clone --branch b1f42fdbcf983c4c45c033d2ac960b1da5ae6a5c --depth 1 https://github.com/deckhouse/k8s-image-availability-exporter.git /src && \
    go get -d -v ./... && \
    CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static"' -o /k8s-image-availability-exporter main.go

FROM $BASE_ALPINE
COPY --from=artifact /k8s-image-availability-exporter /k8s-image-availability-exporter
ENTRYPOINT ["/k8s-image-availability-exporter"]
