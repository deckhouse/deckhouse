ARG BASE_DISTROLESS
ARG BASE_GOLANG_21_ALPINE

# Based on https://github.com/deckhouse/k8s-image-availability-exporter/blob/master/Dockerfile
FROM $BASE_GOLANG_21_ALPINE as artifact

ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}

ARG GOPROXY
ENV GOPROXY=${GOPROXY}

WORKDIR /src
ENV GOARCH=amd64
RUN apk add patch git
RUN git clone --depth 1 --branch v0.7.0 ${SOURCE_REPO}/deckhouse/k8s-image-availability-exporter.git .
COPY patches/001-support-legacy-annotation.patch /src/
RUN patch -p1 < 001-support-legacy-annotation.patch
RUN CGO_ENABLED=0 go build -a -ldflags '-s -w -extldflags "-static"' -o /k8s-image-availability-exporter main.go && \
    chown -R 64535:64535 /k8s-image-availability-exporter && \
    chmod 0700 /k8s-image-availability-exporter

FROM $BASE_DISTROLESS
COPY --from=artifact /k8s-image-availability-exporter /k8s-image-availability-exporter
ENTRYPOINT ["/k8s-image-availability-exporter"]
