ARG BASE_ALPINE
ARG BASE_GOLANG_16_ALPINE
FROM $BASE_GOLANG_16_ALPINE as artifact
WORKDIR /src/
COPY main.go go.mod go.sum /src/
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o prometheus-reverse-proxy main.go

FROM $BASE_ALPINE
COPY --from=artifact /src/prometheus-reverse-proxy /prometheus-reverse-proxy
ENTRYPOINT [ "/prometheus-reverse-proxy" ]
