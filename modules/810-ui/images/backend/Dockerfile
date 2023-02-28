ARG BASE_ALPINE
ARG BASE_GOLANG_19_ALPINE

FROM $BASE_GOLANG_19_ALPINE as artifact
# RUN apk add --update curl gcc musl-dev
# RUN mkdir -p $(go env GOPATH)/bin && curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
#   | sh -s -- -b $(go env GOPATH)/bin v1.51.1 && ls $(go env GOPATH)/bin

# install dependencies
ADD /go.mod /app/go.mod
WORKDIR /app
RUN go mod download

# lint
ADD / /app
# RUN $(go env GOPATH)/bin/golangci-lint run ./...

# build
# RUN go build -ldflags "-linkmode external -extldflags '-static'" -o /backend .
RUN go build -o /backend .

FROM $BASE_ALPINE
COPY --from=artifact /backend /backend
