ARG BASE_GOLANG_20_ALPINE
ARG BASE_DISTROLESS

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
