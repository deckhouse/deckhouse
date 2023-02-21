ARG BASE_ALPINE
ARG BASE_GOLANG_19_ALPINE

# Based on https://github.com/deckhouse/k8s-image-availability-exporter/blob/master/Dockerfile
FROM $BASE_GOLANG_19_ALPINE as artifact
WORKDIR /src
ENV GOARCH=amd64
RUN wget https://github.com/deckhouse/k8s-image-availability-exporter/tarball/44f18a343c3266be366faea4905ef91093aa0039 -O - | tar -xz --strip-components=1 && \
    go get -d -v ./... && \
    CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static"' -o /k8s-image-availability-exporter main.go

FROM $BASE_ALPINE
COPY --from=artifact /k8s-image-availability-exporter /k8s-image-availability-exporter
ENTRYPOINT ["/k8s-image-availability-exporter"]
