ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE
FROM $BASE_GOLANG_ALPINE as artifact
WORKDIR /src/
COPY main.go go.mod go.sum /src/
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o metadata-exporter main.go

FROM $BASE_ALPINE
COPY --from=artifact /src/metadata-exporter /metadata-exporter
RUN apk add --no-cache curl; find /var/cache/apk/ -type f -delete
ENTRYPOINT [ "/metadata-exporter" ]
