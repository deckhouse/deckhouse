ARG BASE_ALPINE
# controller artifact
ARG BASE_GOLANG_16_BUSTER
FROM $BASE_GOLANG_16_BUSTER as artifact
WORKDIR /src/
COPY patches/lua-info.patch /
COPY patches/makefile.patch /
COPY patches/healthcheck.patch /
COPY patches/pod-ip.patch /
COPY patches/deny-invalid-auth-locations.patch /
ENV GOARCH=amd64
RUN apt-get update && apt-get install -y --no-install-recommends git mercurial patch && \
    git clone --branch controller-v0.49.3 --depth 1 https://github.com/kubernetes/ingress-nginx.git /src && \
    patch -p1 < /lua-info.patch && \
    patch -p1 < /makefile.patch && \
    patch -p1 < /pod-ip.patch && \
    patch -p1 < /healthcheck.patch && \
    patch -p1 < /deny-invalid-auth-locations.patch && \
    make GO111MODULE=on USE_DOCKER=false build

# luarocks assets for luajit artifact
FROM quay.io/kubernetes-ingress-controller/nginx-ingress-controller:0.26.1@sha256:d0b22f715fcea5598ef7f869d308b55289a3daaa12922fa52a1abf17703c88e7 as controller_0_26_1
USER root
RUN apt-get update \
  && apt-get install -y --no-install-recommends patch gcc build-essential \
  && luarocks install lua-protobuf 0.3.2-0 \
  && luarocks install lua-iconv 7-3

# IngressNginxController docker image
FROM registry.k8s.io/ingress-nginx/controller:v0.49.3@sha256:35fe394c82164efa8f47f3ed0be981b3f23da77175bbb8268a9ae438851c8324 as controller_0_49_3

# Final image
FROM $BASE_ALPINE
# Based on https://github.com/kubernetes/ingress-nginx/blob/controller-v0.49.3/images/nginx/rootfs/Dockerfile
# Based on https://github.com/kubernetes/ingress-nginx/blob/controller-v0.49.3/rootfs/Dockerfile
ENV PATH=$PATH:/usr/local/luajit/bin:/usr/local/nginx/sbin:/usr/local/nginx/bin

ENV LUA_PATH="/usr/local/share/luajit-2.1.0-beta3/?.lua;/usr/local/share/lua/5.1/?.lua;/usr/local/lib/lua/?.lua;;"
ENV LUA_CPATH="/usr/local/lib/lua/?/?.so;/usr/local/lib/lua/?.so;;"
# Fix for openrestry luarocks paths
ENV PATH=$PATH:/usr/local/openresty/luajit/bin:/usr/local/openresty/nginx/sbin:/usr/local/openresty/bin
ENV LUA_PATH=$LUA_PATH;/usr/local/openresty/site/lualib/?.ljbc;/usr/local/openresty/site/lualib/?/init.ljbc;/usr/local/openresty/lualib/?.ljbc;/usr/local/openresty/lualib/?/init.ljbc;/usr/local/openresty/site/lualib/?.lua;/usr/local/openresty/site/lualib/?/init.lua;/usr/local/openresty/lualib/?.lua;/usr/local/openresty/lualib/?/init.lua;./?.lua;/usr/local/openresty/luajit/share/luajit-2.1.0-beta3/?.lua;/usr/local/share/lua/5.1/?.lua;/usr/local/share/lua/5.1/?/init.lua;/usr/local/openresty/luajit/share/lua/5.1/?.lua;/usr/local/openresty/luajit/share/lua/5.1/?/init.lua;/usr/local/lib/lua/?.lua
ENV LUA_CPATH=$LUA_CPATH;/usr/local/openresty/site/lualib/?.so;/usr/local/openresty/lualib/?.so;./?.so;/usr/local/lib/lua/5.1/?.so;/usr/local/openresty/luajit/lib/lua/5.1/?.so;/usr/local/lib/lua/5.1/loadall.so;/usr/local/openresty/luajit/lib/lua/5.1/?.so

RUN adduser -S -D -H -u 101 -h /usr/local/nginx \
    -s /sbin/nologin -G www-data -g www-data www-data

COPY --from=controller_0_49_3 /usr/local /usr/local
COPY --from=controller_0_49_3 /opt /opt
COPY --from=controller_0_49_3 --chown=www-data:www-data /etc /etc
COPY --from=controller_0_49_3 --chown=www-data:www-data /dbg /dbg
COPY --from=controller_0_49_3 --chown=www-data:www-data /nginx-ingress-controller /nginx-ingress-controller
COPY --from=controller_0_49_3 --chown=www-data:www-data /wait-shutdown /wait-shutdown
COPY --from=artifact /src/rootfs/bin/amd64/nginx-ingress-controller /src/rootfs/bin/amd64/dbg /
# We use 0.26 controller image to copy lua libraries because is easier to install them on ubuntu
COPY --from=controller_0_26_1 /usr/local/openresty/luajit /usr/local/openresty/luajit
COPY patches/balancer-lua.patch /
COPY patches/nginx-tpl.patch /
COPY patches/auth-cookie-always.patch /
COPY rootfs /

RUN apk update \
  && apk upgrade \
  && apk add -U --no-cache \
    bash \
    openssl \
    pcre \
    zlib \
    geoip \
    curl ca-certificates \
    patch \
    yajl \
    lmdb \
    libxml2 \
    libmaxminddb \
    yaml-cpp \
    dumb-init \
    nano \
    tzdata \
    diffutils \
    libcap \
  && ln -s /usr/local/nginx/sbin/nginx /sbin/nginx \
  && bash -eu -c ' \
  writeDirs=( \
    /var/lib/nginx/body \
    /var/lib/nginx/fastcgi \
    /var/lib/nginx/proxy \
    /var/lib/nginx/scgi \
    /var/lib/nginx/uwsgi \
    /etc/ingress-controller \
    /etc/ingress-controller/ssl \
    /etc/ingress-controller/auth \
    /var/log/audit \
    /var/log \
    /var/log/nginx \
  ); \
  for dir in "${writeDirs[@]}"; do \
    mkdir -p ${dir}; \
    chown -R www-data.www-data ${dir}; \
  done' \
  && setcap    cap_net_bind_service=+ep /nginx-ingress-controller \
  && setcap -v cap_net_bind_service=+ep /nginx-ingress-controller \
  && setcap    cap_net_bind_service=+ep /usr/local/nginx/sbin/nginx \
  && setcap -v cap_net_bind_service=+ep /usr/local/nginx/sbin/nginx \
# Create symlinks to redirect nginx logs to stdout and stderr docker log collector
  &&  ln -sf /dev/stdout /var/log/nginx/access.log \
  && ln -sf /dev/stderr /var/log/nginx/error.log \
  && apk del libcap \
  && cd / \
  && patch -p1 < /balancer-lua.patch \
  && patch -p1 < /nginx-tpl.patch \
  && patch -p1 < /auth-cookie-always.patch

WORKDIR /
USER www-data
EXPOSE 80 443
ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/nginx-ingress-controller"]

