ARG BASE_GOLANG_19_ALPINE
ARG BASE_SCRATCH
FROM $BASE_GOLANG_19_ALPINE as artifact
COPY go.* /app/
COPY *.go /app/
WORKDIR /app
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o alerts-receiver alerts-receiver.go alertstore.go config.go

FROM $BASE_SCRATCH
COPY --from=artifact /app/alerts-receiver /alerts-receiver
ENTRYPOINT [ "/alerts-receiver" ]
