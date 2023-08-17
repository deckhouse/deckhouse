ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE
FROM $BASE_GOLANG_20_ALPINE as artifact
WORKDIR /src/
COPY /app/ /src/
RUN apk add --no-cache git && \
    GOPROXY=https://proxy.golang.org GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" ./cmd/crowd-basic-auth-proxy

RUN chown 64535:64535 crowd-basic-auth-proxy
RUN chmod 0700 crowd-basic-auth-proxy

FROM $BASE_DISTROLESS
COPY --from=artifact /src/crowd-basic-auth-proxy /crowd-basic-auth-proxy
ENTRYPOINT [ "/crowd-basic-auth-proxy" ]
