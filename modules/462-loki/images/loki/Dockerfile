ARG BASE_GOLANG_20_ALPINE
ARG BASE_DISTROLESS
# FROM grafana/loki:2.7.3@sha256:8e3abbd89173066721fa07bddfee1c1a7a8fe59bed5b00a2fa09d2b3cef8758c as artifact
FROM $BASE_GOLANG_20_ALPINE as artifact

RUN apk add --no-cache make git
RUN git clone --depth 1 --branch v2.7.3 https://github.com/grafana/loki.git /loki
WORKDIR /loki/
RUN make loki

FROM $BASE_DISTROLESS
COPY --from=artifact /loki/loki /usr/bin
RUN mkdir -p /loki

EXPOSE 3100
ENTRYPOINT ["/usr/bin/loki"]
