ARG BASE_ALPINE
ARG BASE_GOLANG_17_ALPINE

# Based on https://github.com/deckhouse/k8s-image-availability-exporter/blob/master/Dockerfile
FROM $BASE_GOLANG_17_ALPINE as artifact
WORKDIR /src
ENV GOARCH=amd64
RUN wget --no-check-certificate https://github.com/deckhouse/k8s-image-availability-exporter/tarball/51e508a7fc0727556780b34f6e54ee4ef9706816 -O - | tar -xz --strip-components=1 && \
    go get -d -v ./... && \
    CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static"' -o /k8s-image-availability-exporter main.go

FROM $BASE_ALPINE
COPY --from=artifact /k8s-image-availability-exporter /k8s-image-availability-exporter
ENTRYPOINT ["/k8s-image-availability-exporter"]
