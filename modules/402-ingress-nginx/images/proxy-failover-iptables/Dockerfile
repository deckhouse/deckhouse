ARG BASE_ALPINE

ARG BASE_GOLANG_19_ALPINE
FROM $BASE_GOLANG_19_ALPINE as artifact
WORKDIR /src/
COPY main.go go.mod go.sum /src/
RUN apk add --no-cache git && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o iptables-loop main.go

FROM $BASE_ALPINE

RUN apk update && apk add --no-cache iptables && rm -rf /var/cache/apk/*

COPY --from=artifact /src/iptables-loop /

COPY iptables-wrapper-installer.sh /
RUN /iptables-wrapper-installer.sh

ENTRYPOINT [ "/iptables-loop" ]
