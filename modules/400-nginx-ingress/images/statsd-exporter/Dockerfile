FROM golang:1.11-alpine
WORKDIR /go/src/github.com/prometheus/statsd_exporter
# latest master/HEAD
RUN wget https://github.com/prometheus/statsd_exporter/archive/040760ec22f3d8f922da8839ca941586c34f8df4.tar.gz -O - | \
    tar -xz --strip-components=1 && \
    apk --update add make git
ADD 164.patch .
RUN patch -p1 < 164.patch && \
    make build

FROM alpine:3.8
COPY --from=0 /go/src/github.com/prometheus/statsd_exporter/statsd_exporter /bin/
COPY rootfs /
ENTRYPOINT [ "/bin/statsd_exporter" ]
CMD ["--statsd.listen-tcp=", "--statsd.listen-udp=127.0.0.1:9125", "--web.listen-address=127.0.0.1:9102", "--statsd.mapping-config=/etc/statsd_mapping.conf"]
