ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE
FROM $BASE_GOLANG_ALPINE
WORKDIR /src/
COPY main.go go.mod go.sum /src/
RUN apk add --no-cache git && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o sts-pods-hosts-appender-webhook main.go

FROM $BASE_ALPINE
COPY --from=0 /src/sts-pods-hosts-appender-webhook /sts-pods-hosts-appender-webhook
USER nobody
