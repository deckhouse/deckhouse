ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE_DEV
FROM $BASE_GOLANG_20_ALPINE_DEV as artifact
ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64
WORKDIR /src

RUN git clone --depth 1 --branch v7.5.1 ${SOURCE_REPO}/oauth2-proxy/oauth2-proxy.git .
ADD patches/cookie-refresh.patch patches/remove-groups.patch /
RUN patch -p1 < /cookie-refresh.patch && \
  patch -p1 < /remove-groups.patch && \
  go get golang.org/x/net@v0.17.0 && \
  go get google.golang.org/grpc@v1.56.3 && \
  go mod tidy && \
  go build -ldflags '-s -w' -o oauth2-proxy github.com/oauth2-proxy/oauth2-proxy/v7

RUN chown 64535:64535 oauth2-proxy
RUN chmod 0700 oauth2-proxy

WORKDIR /url-exec-prober-src
COPY url-exec-prober/ .
RUN go build -ldflags '-s -w' -o url-exec-prober main.go

RUN chown 64535:64535 url-exec-prober
RUN chmod 0700 url-exec-prober

FROM $BASE_DISTROLESS
COPY --from=artifact /src/oauth2-proxy /bin/oauth2_proxy
COPY --from=artifact /url-exec-prober-src/url-exec-prober /

ENTRYPOINT [ "/bin/oauth2_proxy" ]
CMD [ "--upstream=http://0.0.0.0:8080/", "--http-address=0.0.0.0:4180" ]
