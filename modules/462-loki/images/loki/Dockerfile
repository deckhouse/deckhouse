ARG BASE_GOLANG_20_ALPINE
ARG BASE_DISTROLESS

FROM $BASE_GOLANG_20_ALPINE as artifact

RUN apk add --no-cache make git
RUN git clone --depth 1 --branch v2.7.3 https://github.com/grafana/loki.git /loki
WORKDIR /loki/
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o loki cmd/loki/main.go

FROM $BASE_DISTROLESS
COPY --from=artifact /loki/loki /usr/bin
WORKDIR /loki/

EXPOSE 3100
ENTRYPOINT ["/usr/bin/loki"]
