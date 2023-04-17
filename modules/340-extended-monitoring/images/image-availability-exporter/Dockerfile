ARG BASE_ALPINE
ARG BASE_GOLANG_19_ALPINE

# Based on https://github.com/deckhouse/k8s-image-availability-exporter/blob/master/Dockerfile
FROM $BASE_GOLANG_19_ALPINE as artifact
WORKDIR /src
ENV GOARCH=amd64
RUN apk add patch
COPY patches/001-support-legacy-annotation.patch /src/
RUN wget https://github.com/deckhouse/k8s-image-availability-exporter/tarball/v0.5.0 -O - | tar -xz --strip-components=1 && \
    patch -p1 < 001-support-legacy-annotation.patch && \
    CGO_ENABLED=0 go build -a -ldflags '-s -w -extldflags "-static"' -o /k8s-image-availability-exporter main.go

FROM $BASE_ALPINE
COPY --from=artifact /k8s-image-availability-exporter /k8s-image-availability-exporter
ENTRYPOINT ["/k8s-image-availability-exporter"]
