FROM golang:1.12-alpine3.10
WORKDIR /src/
COPY main.go go.mod go.sum /src/
RUN apk add --no-cache git && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ingress-conversion-webhook main.go

FROM alpine:3.10
COPY --from=0 /src/ingress-conversion-webhook /ingress-conversion-webhook
USER nobody
ENTRYPOINT [ "/ingress-conversion-webhook" ]
