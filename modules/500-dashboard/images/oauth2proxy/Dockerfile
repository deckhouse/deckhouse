FROM golang:1.9-alpine3.7
WORKDIR /go/src/github.com/bitly/oauth2_proxy

RUN apk --update add make git build-base curl bash ca-certificates wget \
 && update-ca-certificates \
 && curl -sSO https://raw.githubusercontent.com/pote/gpm/v1.4.0/bin/gpm \
 && chmod +x gpm \
 && mv gpm /usr/local/bin
RUN git clone https://github.com/bitly/oauth2_proxy.git . \
 && git checkout bfda078caa55958cc37dcba39e57fc37f6a3c842 
ADD rd.patch .
RUN patch -p1 < rd.patch \
 && ./dist.sh

FROM alpine:3.7
RUN apk --update add curl bash  ca-certificates && update-ca-certificates
COPY --from=0 /go/src/github.com/bitly/oauth2_proxy/dist/ /bin/

EXPOSE 8080 4180
ENTRYPOINT [ "/bin/oauth2_proxy" ]
CMD [ "--upstream=http://0.0.0.0:8080/", "--http-address=0.0.0.0:4180" ]

