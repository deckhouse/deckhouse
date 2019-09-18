FROM golang:1.12-alpine3.10
WORKDIR /src/
COPY main.go go.mod go.sum /src/
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o user-authz-webhook main.go

FROM alpine:3.10
COPY --from=0 /src/user-authz-webhook /user-authz-webhook
RUN apk add --no-cache curl; find /var/cache/apk/ -type f -delete
ENTRYPOINT [ "/user-authz-webhook" ]
