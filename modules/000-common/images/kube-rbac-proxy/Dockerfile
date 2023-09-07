ARG BASE_DISTROLESS
ARG BASE_GOLANG_16_ALPINE

FROM $BASE_GOLANG_16_ALPINE as build

ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /src/
RUN apk add --no-cache git patch make && \
    git clone --depth 1 --branch v0.11.0 ${SOURCE_REPO}/brancz/kube-rbac-proxy/kube-rbac-proxy.git /src
COPY patches/stale-cache.patch /src
COPY patches/config.patch /src
COPY patches/livez.patch /src
COPY patches/insecure-upstream.patch /src
COPY patches/preserve-auth-header.patch /src
COPY patches/secure-listen-address.patch /src

RUN patch -p1 < /src/stale-cache.patch && \
    patch -p1 < /src/config.patch && \
    patch -p1 < /src/livez.patch && \
    patch -p1 < /src/insecure-upstream.patch && \
    patch -p1 < /src/preserve-auth-header.patch && \
    patch -p1 < /src/secure-listen-address.patch && \
    make build && \
    cp /src/_output/kube-rbac-proxy-linux-$(go env GOARCH) /kube-rbac-proxy && \
    chown 64535:64535 /kube-rbac-proxy && \
    chmod 0700 /kube-rbac-proxy

FROM $BASE_DISTROLESS
COPY --from=build /kube-rbac-proxy /kube-rbac-proxy
ENTRYPOINT ["/kube-rbac-proxy", "--tls-cipher-suites", "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,TLS_RSA_WITH_AES_128_CBC_SHA,TLS_RSA_WITH_AES_256_CBC_SHA,TLS_RSA_WITH_AES_128_GCM_SHA256,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA"]
EXPOSE 8080
