ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE

FROM $BASE_GOLANG_ALPINE as builder
RUN apk add --update gcc musl-dev jq-dev oniguruma-dev curl
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
  | sh -s -- -b $(go env GOPATH)/bin v1.32.2
# install dependencies
ADD /go.mod /app/go.mod
WORKDIR /app
RUN go mod download
# lint
ADD / /app
RUN $(go env GOPATH)/bin/golangci-lint run ./...
# build
RUN go build -ldflags "-linkmode external -extldflags '-static'" -o /upmeter ./cmd/upmeter

FROM $BASE_ALPINE
# for debug
RUN apk add --update sqlite
COPY --from=builder /app/pkg/upmeter/db/migrations /data/migrations
COPY --from=builder /upmeter /upmeter
