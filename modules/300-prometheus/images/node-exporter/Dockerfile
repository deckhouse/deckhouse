FROM golang:1.9-alpine3.7
WORKDIR /go/src/github.com/prometheus/node_exporter

RUN wget https://github.com/prometheus/node_exporter/archive/v0.15.2.tar.gz -O - | tar -xz --strip-components=1
RUN apk --update add make git build-base
ADD rootfs.patch .
RUN patch -p1 < rootfs.patch && \
    make build

FROM alpine:3.7
COPY --from=0 /go/src/github.com/prometheus/node_exporter/node_exporter /bin/
ENTRYPOINT [ "/bin/node_exporter" ]
