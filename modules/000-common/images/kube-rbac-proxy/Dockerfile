ARG BASE_ALPINE
ARG BASE_GOLANG_16_ALPINE

FROM $BASE_GOLANG_16_ALPINE as build
WORKDIR /src/
RUN apk add --no-cache git patch make
RUN wget https://github.com/brancz/kube-rbac-proxy/archive/v0.11.0.tar.gz -O - | tar -xz --strip-components=1 -C /src
COPY patches/stale-cache.patch /src
COPY patches/config.patch /src
COPY patches/livez.patch /src
RUN patch -p1 < /src/stale-cache.patch && \
    patch -p1 < /src/config.patch && \
    patch -p1 < /src/livez.patch && \
    make build && \
    cp /src/_output/kube-rbac-proxy-linux-$(go env GOARCH) /kube-rbac-proxy

FROM $BASE_ALPINE
RUN apk add -U --no-cache ca-certificates && rm -rf /var/cache/apk/*
COPY --from=build /kube-rbac-proxy /kube-rbac-proxy
COPY entrypoint.sh /
ENTRYPOINT ["/entrypoint.sh"]
EXPOSE 8080
