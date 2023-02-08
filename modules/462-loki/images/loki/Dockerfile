ARG BASE_ALPINE
FROM grafana/loki:2.7.3@sha256:8e3abbd89173066721fa07bddfee1c1a7a8fe59bed5b00a2fa09d2b3cef8758c as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/bin/loki /usr/bin
RUN mkdir -p /loki && chown nobody:nobody /loki
USER nobody

EXPOSE 3100
ENTRYPOINT ["/usr/bin/loki"]
