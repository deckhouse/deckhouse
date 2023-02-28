ARG BASE_ALPINE
ARG BASE_GOLANG_16_ALPINE

FROM $BASE_GOLANG_16_ALPINE as artifact
RUN apk add --update gcc musl-dev jq-dev oniguruma-dev curl
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
  | sh -s -- -b $(go env GOPATH)/bin v1.46.2
# install dependencies
ADD /go.mod /app/go.mod
WORKDIR /app
RUN go mod download
# lint
ADD / /app
RUN $(go env GOPATH)/bin/golangci-lint run ./...
# build
RUN go build -ldflags "-linkmode external -extldflags '-static'" -o /upmeter ./cmd/upmeter
# add migrator, outside of go module
RUN go install -tags 'sqlite3' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.15 \
  && mv $(go env GOPATH)/bin/migrate /migrate

FROM $BASE_ALPINE
# sqlite for debug
RUN apk add --update sqlite tree
COPY --from=artifact /app/pkg/db/migrations/agent  /data/migrations/agent
COPY --from=artifact /app/pkg/db/migrations/server /data/migrations/server
COPY --from=artifact /migrate /migrate
COPY --from=artifact /upmeter /upmeter
