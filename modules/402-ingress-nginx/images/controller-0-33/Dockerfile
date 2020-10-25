ARG BASE_GOLANG_BUSTER
FROM $BASE_GOLANG_BUSTER
WORKDIR /src/
COPY lua-info.patch /
ENV GOARCH=amd64
RUN apt-get update && apt-get install -y --no-install-recommends git mercurial patch && \
    git clone --branch controller-0.33.0 --depth 1 https://github.com/kubernetes/ingress-nginx.git /src && \
    patch -p1 < /lua-info.patch && \
    # todo: Разобраться, почему игнорируется vendor
    make GO111MODULE=on USE_DOCKER=false build

# luarocks assets for luajit artifact
FROM quay.io/kubernetes-ingress-controller/nginx-ingress-controller:0.26.1
USER root
RUN apt-get update \
  && apt-get install -y --no-install-recommends patch gcc build-essential \
  && luarocks install lua-protobuf 0.3.2-0 \
  && luarocks install lua-iconv 7-3

# IngressNginxController docker image
FROM quay.io/kubernetes-ingress-controller/nginx-ingress-controller:0.33.0
COPY --from=0 /src/rootfs/bin/amd64/nginx-ingress-controller /src/rootfs/bin/amd64/dbg /
COPY --from=1 /usr/local/openresty/luajit /usr/local/openresty/luajit
COPY balancer-lua.patch /
USER root
WORKDIR /
RUN apk add --no-cache patch && patch -p1 < /balancer-lua.patch
USER www-data
# Fix for openrestry luarocks paths
ENV PATH=$PATH:/usr/local/openresty/luajit/bin:/usr/local/openresty/nginx/sbin:/usr/local/openresty/bin
ENV LUA_PATH=$LUA_PATH;/usr/local/openresty/site/lualib/?.ljbc;/usr/local/openresty/site/lualib/?/init.ljbc;/usr/local/openresty/lualib/?.ljbc;/usr/local/openresty/lualib/?/init.ljbc;/usr/local/openresty/site/lualib/?.lua;/usr/local/openresty/site/lualib/?/init.lua;/usr/local/openresty/lualib/?.lua;/usr/local/openresty/lualib/?/init.lua;./?.lua;/usr/local/openresty/luajit/share/luajit-2.1.0-beta3/?.lua;/usr/local/share/lua/5.1/?.lua;/usr/local/share/lua/5.1/?/init.lua;/usr/local/openresty/luajit/share/lua/5.1/?.lua;/usr/local/openresty/luajit/share/lua/5.1/?/init.lua;/usr/local/lib/lua/?.lua
ENV LUA_CPATH=$LUA_CPATH;/usr/local/openresty/site/lualib/?.so;/usr/local/openresty/lualib/?.so;./?.so;/usr/local/lib/lua/5.1/?.so;/usr/local/openresty/luajit/lib/lua/5.1/?.so;/usr/local/lib/lua/5.1/loadall.so;/usr/local/openresty/luajit/lib/lua/5.1/?.so

COPY rootfs /
