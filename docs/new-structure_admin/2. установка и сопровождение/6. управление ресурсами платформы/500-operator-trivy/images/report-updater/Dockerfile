ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE

FROM $BASE_GOLANG_20_ALPINE as builder
WORKDIR /src/report-updater
COPY main.go go.mod go.sum /src/report-updater/
COPY cache /src/report-updater/cache/
COPY web /src/report-updater/web/

RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test ./...
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o report-updater main.go

FROM $BASE_DISTROLESS
COPY --from=builder /src/report-updater/report-updater /report-updater
ENTRYPOINT [ "/report-updater" ]
