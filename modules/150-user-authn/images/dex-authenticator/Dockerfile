ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE
FROM $BASE_GOLANG_20_ALPINE as artifact
ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}

WORKDIR /src

# Download tools
RUN apk --update add make git build-base curl bash ca-certificates wget \
 && update-ca-certificates
RUN git clone --depth 1 --branch v7.2.0 ${SOURCE_REPO}/oauth2-proxy/oauth2-proxy.git .
ADD patches/cookie-refresh.patch patches/remove-groups.patch /
RUN patch -p1 < /cookie-refresh.patch \
  && patch -p1 < /remove-groups.patch \
  && GOPROXY=https://proxy.golang.org CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w' -o oauth2-proxy github.com/oauth2-proxy/oauth2-proxy/v7

RUN chown 64535:64535 oauth2-proxy
RUN chmod 0700 oauth2-proxy

FROM $BASE_DISTROLESS
COPY --from=artifact /src/oauth2-proxy /bin/oauth2_proxy

ENTRYPOINT [ "/bin/oauth2_proxy" ]
CMD [ "--upstream=http://0.0.0.0:8080/", "--http-address=0.0.0.0:4180" ]
