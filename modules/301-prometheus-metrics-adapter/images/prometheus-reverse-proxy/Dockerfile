ARG BASE_ALPINE
ARG BASE_GOLANG_17_ALPINE
FROM $BASE_GOLANG_17_ALPINE as artifact
WORKDIR /src/
COPY /app/ /src/
RUN apk add --no-cache git && \
    go test ./... && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" ./cmd/prometheus-reverse-proxy

FROM $BASE_ALPINE
COPY --from=artifact /src/prometheus-reverse-proxy /prometheus-reverse-proxy
ENTRYPOINT [ "/prometheus-reverse-proxy" ]
