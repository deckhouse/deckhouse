FROM golang:1.12-alpine3.10
WORKDIR /src/
COPY main.go go.mod go.sum /src/
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o prometheus-reverse-proxy main.go

FROM alpine:3.10
COPY --from=0 /src/prometheus-reverse-proxy /prometheus-reverse-proxy
ENTRYPOINT [ "/prometheus-reverse-proxy" ]
