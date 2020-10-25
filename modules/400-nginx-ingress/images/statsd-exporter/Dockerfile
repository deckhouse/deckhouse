ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE
FROM $BASE_GOLANG_ALPINE
WORKDIR /go/src/github.com/prometheus/statsd_exporter
ENV USER=root
RUN apk add --no-cache make git curl && \
    wget https://github.com/prometheus/statsd_exporter/archive/v0.9.0-rc3.tar.gz -O - | \
    tar -xz --strip-components=1 && \
    make build

FROM $BASE_ALPINE
COPY --from=0 /go/src/github.com/prometheus/statsd_exporter/statsd_exporter /bin/
COPY rootfs /
ENTRYPOINT [ "/bin/statsd_exporter" ]
CMD ["--statsd.listen-tcp=", "--statsd.listen-udp=127.0.0.1:9125", "--web.listen-address=127.0.0.1:9102", "--statsd.mapping-config=/etc/statsd_mapping.conf"]
