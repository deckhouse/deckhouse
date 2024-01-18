ARG BASE_ALPINE
# controller artifact
ARG BASE_GOLANG_21_BULLSEYE
FROM $BASE_GOLANG_21_BULLSEYE as artifact
WORKDIR /src/
COPY patches/lua-info.patch /
COPY patches/makefile.patch /
COPY patches/healthcheck.patch /
COPY patches/metrics-SetSSLExpireTime.patch /
COPY patches/util.patch /
COPY patches/fix-cleanup.patch /
ENV GOARCH=amd64
RUN apt-get update && apt-get install -y --no-install-recommends git mercurial patch && \
    git clone --branch controller-v1.9.4 --depth 1 https://github.com/kubernetes/ingress-nginx.git /src && \
    patch -p1 < /lua-info.patch && \
    patch -p1 < /makefile.patch && \
    patch -p1 < /healthcheck.patch && \
    patch -p1 < /metrics-SetSSLExpireTime.patch && \
    patch -p1 < /util.patch && \
    patch -p1 < /fix-cleanup.patch && \
    make GO111MODULE=on USE_DOCKER=false build

# luarocks assets for luajit artifact
FROM quay.io/kubernetes-ingress-controller/nginx-ingress-controller:0.26.1@sha256:d0b22f715fcea5598ef7f869d308b55289a3daaa12922fa52a1abf17703c88e7 as controller_0_26_1
USER root
RUN apt-get update \
  && apt-get install -y --no-install-recommends patch gcc build-essential \
  && luarocks install lua-protobuf 0.4.1-1 \
  && luarocks install lua-iconv 7-3

# IngressNginxController docker image
FROM registry.k8s.io/ingress-nginx/controller:v1.9.3@sha256:8fd21d59428507671ce0fb47f818b1d859c92d2ad07bb7c947268d433030ba98 as controller_image

# Final image
FROM $BASE_ALPINE
# Based on https://github.com/kubernetes/ingress-nginx/blob/controller-v1.0.4/images/nginx/rootfs/Dockerfile
# Based on https://github.com/kubernetes/ingress-nginx/blob/controller-v1.0.4/rootfs/Dockerfile
ENV PATH=$PATH:/usr/local/luajit/bin:/usr/local/nginx/sbin:/usr/local/nginx/bin

ENV LUA_PATH="/usr/local/share/luajit-2.1.0-beta3/?.lua;/usr/local/share/lua/5.1/?.lua;/usr/local/lib/lua/?.lua;;"
ENV LUA_CPATH="/usr/local/lib/lua/?/?.so;/usr/local/lib/lua/?.so;;"
# Fix for openrestry luarocks paths
ENV PATH=$PATH:/usr/local/openresty/luajit/bin:/usr/local/openresty/nginx/sbin:/usr/local/openresty/bin
ENV LUA_PATH=$LUA_PATH;/usr/local/openresty/site/lualib/?.ljbc;/usr/local/openresty/site/lualib/?/init.ljbc;/usr/local/openresty/lualib/?.ljbc;/usr/local/openresty/lualib/?/init.ljbc;/usr/local/openresty/site/lualib/?.lua;/usr/local/openresty/site/lualib/?/init.lua;/usr/local/openresty/lualib/?.lua;/usr/local/openresty/lualib/?/init.lua;./?.lua;/usr/local/openresty/luajit/share/luajit-2.1.0-beta3/?.lua;/usr/local/share/lua/5.1/?.lua;/usr/local/share/lua/5.1/?/init.lua;/usr/local/openresty/luajit/share/lua/5.1/?.lua;/usr/local/openresty/luajit/share/lua/5.1/?/init.lua;/usr/local/lib/lua/?.lua
ENV LUA_CPATH=$LUA_CPATH;/usr/local/openresty/site/lualib/?.so;/usr/local/openresty/lualib/?.so;./?.so;/usr/local/lib/lua/5.1/?.so;/usr/local/openresty/luajit/lib/lua/5.1/?.so;/usr/local/lib/lua/5.1/loadall.so;/usr/local/openresty/luajit/lib/lua/5.1/?.so

RUN adduser -S -D -H -u 101 -h /usr/local/nginx \
    -s /sbin/nologin -G www-data -g www-data www-data

COPY --from=controller_image /usr/local /usr/local
COPY --from=controller_image /opt /opt
COPY --from=controller_image --chown=www-data:www-data /etc /etc
COPY --from=controller_image --chown=www-data:www-data /wait-shutdown /wait-shutdown
COPY --from=artifact /src/rootfs/bin/amd64/nginx-ingress-controller /src/rootfs/bin/amd64/dbg /
# Get nginx config template from the repository as controller_image containts an invalid one by the time of testing
COPY --from=artifact --chown=www-data:www-data /src/rootfs/etc/nginx/template /etc/nginx/template
# We use 0.26 controller image to copy lua libraries because is easier to install them on ubuntu
COPY --from=controller_0_26_1 /usr/local/openresty/luajit /chroot/usr/local/openresty/luajit
COPY patches/balancer-lua.patch /
COPY patches/nginx-tmpl.patch /
COPY patches/auth-cookie-always.patch /
COPY --chown=www-data:www-data nginx-chroot-wrapper.sh /usr/bin/nginx

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
    util-linux \
  && ln -s /usr/local/nginx/sbin/nginx /sbin/nginx \
  && bash -eu -c ' \
  writeDirs=( \
      /chroot/usr/local/ \
      /chroot/opt/modsecurity/var/log \
      /chroot/opt/modsecurity/var/upload \
      /chroot/opt/modsecurity/var/audit \
      /chroot/var/log/audit \
      /chroot/var/lib/nginx \
      /chroot/var/log/nginx \
      /chroot/var/lib/nginx/body \
      /chroot/var/lib/nginx/fastcgi \
      /chroot/var/lib/nginx/proxy \
      /chroot/var/lib/nginx/scgi \
      /chroot/var/lib/nginx/uwsgi \
      /chroot/tmp/nginx \
  ); \
  for dir in "${writeDirs[@]}"; do \
    mkdir -p ${dir}; \
    chown -R www-data.www-data ${dir}; \
  done' \
# Create opentelemetry.toml file as it doesn't present in controller_image
  && touch /etc/nginx/opentelemetry.toml \
  && chown -R www-data.www-data /etc/nginx/opentelemetry.toml \
  && mkdir -p /chroot/lib /chroot/proc /chroot/usr /chroot/bin /chroot/dev /chroot/run /chroot/etc \
  && cp /etc/passwd /etc/group /chroot/etc/ \
  && cp -a /usr/* /chroot/usr/ \
  && mv /etc/nginx /chroot/etc/ \
  && mv /etc/ingress-controller /chroot/etc/ \
  && cp /lib/ld-musl-* /lib/libcrypto* /lib/libssl* /lib/libz* /chroot/lib/ \
  && echo "/lib:/usr/lib:/usr/local/lib:/modules_mount/etc/nginx/modules/otel" > /chroot/etc/ld-musl-x86_64.path \
  && setcap    cap_sys_chroot,cap_net_bind_service=+ep /nginx-ingress-controller \
  && setcap -v cap_sys_chroot,cap_net_bind_service=+ep /nginx-ingress-controller \
  && setcap    cap_sys_chroot,cap_net_bind_service=+ep /usr/bin/unshare \
  && setcap -v cap_sys_chroot,cap_net_bind_service=+ep /usr/bin/unshare \
  && setcap    cap_net_bind_service=+ep /chroot/usr/local/nginx/sbin/nginx \
  && setcap -v cap_net_bind_service=+ep /chroot/usr/local/nginx/sbin/nginx \
  && setcap    cap_sys_chroot,cap_net_bind_service=+ep /usr/bin/dumb-init \
  && setcap -v cap_sys_chroot,cap_net_bind_service=+ep /usr/bin/dumb-init \
  && ln -sf /chroot/etc/nginx /etc/nginx \
  && ln -sf /chroot/tmp/nginx /tmp/nginx \
  && ln -sf /chroot/etc/ingress-controller /etc/ingress-controller \
# Create symlinks to redirect nginx logs to stdout and stderr docker log collector
  && ln -sf /chroot/var/log/nginx /var/log/nginx \
  && touch /chroot/var/log/nginx/access.log \
  && echo "" > /chroot/etc/resolv.conf \
  && chown -R www-data.www-data /var/log /chroot/var/log /chroot/etc/resolv.conf \
  && mknod -m 0666 /chroot/dev/null c 1 3 \
  && mknod -m 0666 /chroot/dev/random c 1 8 \
  && mknod -m 0666 /chroot/dev/urandom c 1 9 \
  && mknod -m 0666 /chroot/dev/full c 1 7 \
  && mknod -m 0666 /chroot/dev/ptmx c 5 2 \
  && mknod -m 0666 /chroot/dev/zero c 1 5 \
  && mknod -m 0666 /chroot/dev/tty c 5 0 \
  && mkdir -p /chroot/modules_mount \
  && mkdir -p modules_mount \
  && ln -s /modules_mount /chroot/modules_mount \
  && apk del libcap \
  && cd / \
  && patch -p1 < /balancer-lua.patch \
  && patch -p1 < /nginx-tmpl.patch \
  && patch -p1 < /auth-cookie-always.patch \
  && rm -rf /*.patch

COPY --chown=www-data:www-data rootfs/etc /chroot/etc/
WORKDIR /
USER www-data
EXPOSE 80 443
ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/nginx-ingress-controller"]
