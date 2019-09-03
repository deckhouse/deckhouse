FROM golang:1.12-buster

WORKDIR /src/
COPY lua-info.patch /
ENV GOARCH=amd64
RUN apt-get update && apt-get install -y --no-install-recommends git mercurial patch && \
    git clone --branch nginx-0.25.1 --depth 1 https://github.com/kubernetes/ingress-nginx.git /src && \
    patch -p1 < /lua-info.patch && \
    make build


FROM quay.io/kubernetes-ingress-controller/nginx-ingress-controller:0.25.1

COPY --from=0 /src/bin/amd64/nginx-ingress-controller /src/bin/amd64/dbg /

COPY balancer-lua.patch /
USER root
RUN apt-get update && apt-get install -y --no-install-recommends patch && \
    apt-get clean -y && \
    rm -rf \
        /var/cache/debconf/* \
        /var/lib/apt/lists/* \
        /tmp/* \
        /var/tmp/* && \
    cd / && patch -p1 < /balancer-lua.patch
USER www-data

COPY rootfs /
