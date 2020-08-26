FROM golang:1.14.4-alpine3.12
RUN apk add --update gcc musl-dev jq-dev oniguruma-dev
ADD /go.mod /app/go.mod
WORKDIR /app
RUN go mod download
ADD / /app
RUN go build -ldflags "-linkmode external -extldflags '-static'" -o /upmeter ./cmd/upmeter

FROM alpine:3.12
COPY --from=0 /upmeter /upmeter
#ADD /test.db /
