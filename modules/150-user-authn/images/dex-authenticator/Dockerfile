ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE
FROM $BASE_GOLANG_20_ALPINE as artifact
WORKDIR /go/src/github.com/oauth2-proxy/oauth2_proxy

# Download tools
RUN apk --update add make git build-base curl bash ca-certificates wget \
 && update-ca-certificates
RUN git clone https://github.com/oauth2-proxy/oauth2-proxy.git . \
 && git checkout v7.2.0
ADD patches/cookie-refresh.patch patches/remove-groups.patch /
RUN patch -p1 < /cookie-refresh.patch \
  && patch -p1 < /remove-groups.patch \
  && make build

RUN chown 64535:64535 oauth2-proxy
RUN chmod 0700 oauth2-proxy

FROM $BASE_DISTROLESS
COPY --from=artifact /go/src/github.com/oauth2-proxy/oauth2_proxy/oauth2-proxy /bin/oauth2_proxy

EXPOSE 8080 4180
ENTRYPOINT [ "/bin/oauth2_proxy" ]
CMD [ "--upstream=http://0.0.0.0:8080/", "--http-address=0.0.0.0:4180" ]
