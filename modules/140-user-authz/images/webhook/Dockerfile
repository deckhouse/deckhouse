ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE
FROM $BASE_GOLANG_ALPINE
WORKDIR /src/
COPY main.go go.mod go.sum /src/
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o user-authz-webhook main.go

FROM $BASE_ALPINE
COPY --from=0 /src/user-authz-webhook /user-authz-webhook
RUN apk add --no-cache curl; find /var/cache/apk/ -type f -delete
ENTRYPOINT [ "/user-authz-webhook" ]
