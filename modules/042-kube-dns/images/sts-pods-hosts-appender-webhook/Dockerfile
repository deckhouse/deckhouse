ARG BASE_GOLANG_19_ALPINE
ARG BASE_DISTROLESS
FROM $BASE_GOLANG_19_ALPINE as artifact
WORKDIR /src/
COPY main.go go.mod go.sum /src/
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o sts-pods-hosts-appender-webhook main.go

FROM $BASE_DISTROLESS
COPY --from=artifact /src/sts-pods-hosts-appender-webhook /sts-pods-hosts-appender-webhook
