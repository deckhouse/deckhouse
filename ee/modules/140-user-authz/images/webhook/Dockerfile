ARG BASE_ALPINE
ARG BASE_GOLANG_16_ALPINE
FROM $BASE_GOLANG_16_ALPINE as artifact
WORKDIR /src/user-authz-webhook
COPY main.go go.mod go.sum /src/user-authz-webhook/
COPY cache /src/user-authz-webhook/cache/
COPY web /src/user-authz-webhook/web/

RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test ./...
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o user-authz-webhook main.go

FROM $BASE_ALPINE
COPY --from=artifact /src/user-authz-webhook/user-authz-webhook /user-authz-webhook
RUN apk add --no-cache curl; find /var/cache/apk/ -type f -delete
ENTRYPOINT [ "/user-authz-webhook" ]
