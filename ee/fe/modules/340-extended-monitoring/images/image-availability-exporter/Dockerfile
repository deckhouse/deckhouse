ARG BASE_ALPINE
ARG BASE_GOLANG_16_BUSTER

# Based on https://github.com/deckhouse/k8s-image-availability-exporter/blob/master/Dockerfile
FROM $BASE_GOLANG_16_BUSTER as artifact
WORKDIR /src
ENV GOARCH=amd64
RUN apt-get update && \
    apt-get install -y --no-install-recommends git && \
    git clone --branch v0.2.0 --depth 1 https://github.com/deckhouse/k8s-image-availability-exporter.git /src && \
    go get -d -v ./... && \
    CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static"' -o /k8s-image-availability-exporter main.go

FROM $BASE_ALPINE
COPY --from=artifact /k8s-image-availability-exporter /k8s-image-availability-exporter
ENTRYPOINT ["/k8s-image-availability-exporter"]
