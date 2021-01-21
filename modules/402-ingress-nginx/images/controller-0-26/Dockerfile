# controller artifact
ARG BASE_GOLANG_BUSTER
FROM $BASE_GOLANG_BUSTER
WORKDIR /src/
COPY lua-info.patch /
COPY reason.patch /
COPY omit-helm-secrets.patch /
ENV GOARCH=amd64
RUN apt-get update && apt-get install -y --no-install-recommends git mercurial patch && \
    git clone --branch nginx-0.26.1 --depth 1 https://github.com/kubernetes/ingress-nginx.git /src && \
    patch -p1 < /lua-info.patch && \
    patch -p1 < /reason.patch && \
    patch -p1 < /omit-helm-secrets.patch && \
    # todo: Разобраться, почему игнорируется vendor
    make GO111MODULE=on build

# luarocks assets for luajit artifact
FROM quay.io/kubernetes-ingress-controller/nginx-ingress-controller:0.26.1
USER root
RUN apt-get update \
  && apt-get install -y --no-install-recommends patch gcc build-essential \
  && luarocks install lua-protobuf 0.3.2-0 \
  && luarocks install lua-iconv 7-3

# IngressNginxController docker image
FROM quay.io/kubernetes-ingress-controller/nginx-ingress-controller:0.26.1
COPY --from=0 /src/bin/amd64/nginx-ingress-controller /src/bin/amd64/dbg /
COPY --from=1 /usr/local/openresty/luajit /usr/local/openresty/luajit
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
