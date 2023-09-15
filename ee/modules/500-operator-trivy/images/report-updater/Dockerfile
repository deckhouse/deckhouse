ARG BASE_ALPINE
ARG BASE_GOLANG_20_ALPINE
FROM $BASE_GOLANG_20_ALPINE as artifact
WORKDIR /src/report-updater
COPY main.go go.mod go.sum /src/report-updater/
COPY vulndb /src/report-updater/vulndb/
COPY web /src/report-updater/web/

RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test ./...
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o report-updater main.go

FROM $BASE_ALPINE
COPY --from=artifact /src/report-updater/report-updater /report-updater
RUN apk add --no-cache curl; find /var/cache/apk/ -type f -delete
ENTRYPOINT [ "/report-updater" ]
