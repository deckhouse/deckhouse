FROM golang:1.12-alpine3.10
WORKDIR /go/src/github.com/pusher/oauth2_proxy

# Download tools
RUN wget -O $GOPATH/bin/dep https://github.com/golang/dep/releases/download/v0.5.0/dep-linux-amd64
RUN chmod +x $GOPATH/bin/dep

RUN apk --update add make git build-base curl bash ca-certificates wget \
 && update-ca-certificates \
 && curl -sSO https://raw.githubusercontent.com/pote/gpm/v1.4.0/bin/gpm \
 && chmod +x gpm \
 && mv gpm /usr/local/bin
RUN git clone https://github.com/pusher/oauth2_proxy.git . \
 && git checkout v3.2.0
ADD rd.patch .
RUN patch -p1 < rd.patch \
 && ./dist.sh

FROM alpine:3.10
RUN apk --update add curl bash  ca-certificates && update-ca-certificates
COPY --from=0 /go/src/github.com/pusher/oauth2_proxy/dist/ /bin/

EXPOSE 8080 4180
ENTRYPOINT [ "/bin/oauth2_proxy" ]
CMD [ "--upstream=http://0.0.0.0:8080/", "--http-address=0.0.0.0:4180" ]

