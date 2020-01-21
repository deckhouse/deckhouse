FROM golang:1.12-alpine
WORKDIR /go/src/github.com/prometheus/
ENV USER=root
RUN apk add --no-cache make git patch curl && \
    git clone -b "v0.12.2" --single-branch https://github.com/prometheus/statsd_exporter.git
WORKDIR /go/src/github.com/prometheus/statsd_exporter
RUN make common-build

FROM alpine:3.10
COPY --from=0 /go/src/github.com/prometheus/statsd_exporter/statsd_exporter /bin/
COPY rootfs /
ENTRYPOINT [ "/bin/statsd_exporter" ]
CMD ["--statsd.listen-tcp=", "--statsd.listen-udp=127.0.0.1:9125", "--web.listen-address=127.0.0.1:9102", "--statsd.mapping-config=/etc/statsd_mapping.conf"]
