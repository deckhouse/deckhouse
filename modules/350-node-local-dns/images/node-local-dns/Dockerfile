FROM golang:1.11-alpine3.9

WORKDIR /app

COPY src /app

RUN apk add --no-cache git build-base && go build -mod=vendor cmd/main.go

FROM alpine:3.9
COPY --from=0 /app/main /usr/local/bin/node-local-dns
RUN apk add --no-cache iptables
ENTRYPOINT ["/usr/local/bin/node-local-dns"]
