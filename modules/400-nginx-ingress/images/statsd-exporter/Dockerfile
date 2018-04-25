FROM quay.io/prometheus/statsd-exporter:v0.6.0

COPY rootfs /

CMD ["-statsd.listen-tcp=", "-statsd.listen-udp=127.0.0.1:9125", "-web.listen-address=127.0.0.1:9102", "-statsd.mapping-config=/etc/statsd_mapping.conf"]
