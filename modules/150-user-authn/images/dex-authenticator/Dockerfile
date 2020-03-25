FROM golang:1.13-alpine3.10
WORKDIR /go/src/github.com/pusher/oauth2_proxy

# Download tools
RUN wget -O - https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(go env GOPATH)/bin v1.17.1

RUN apk --update add make git build-base curl bash ca-certificates wget \
 && update-ca-certificates
RUN git clone https://github.com/pusher/oauth2_proxy.git . \
 && git checkout v5.0.0
ADD cookie-refresh.patch /
RUN patch -p1 < /cookie-refresh.patch \
  && GO111MODULE=on go mod download \
  && ./configure \
  && make build

FROM alpine:3.10
RUN apk --update add curl bash  ca-certificates && update-ca-certificates
COPY --from=0 /go/src/github.com/pusher/oauth2_proxy/oauth2_proxy /bin/oauth2_proxy

EXPOSE 8080 4180
ENTRYPOINT [ "/bin/oauth2_proxy" ]
CMD [ "--upstream=http://0.0.0.0:8080/", "--http-address=0.0.0.0:4180" ]
