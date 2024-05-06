ARG BASE_ALT
ARG BASE_ALT_DEV
ARG BASE_ALPINE_DEV
ARG BASE_GOLANG_21_BULLSEYE_DEV
ARG CONTROLLER_BRANCH=controller-v1.10.1

FROM $BASE_ALT as base-alt

# Build dumb-init binary
FROM $BASE_ALPINE_DEV as dumb-init-builder
ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}
RUN git clone --branch v1.2.5 --depth 1 ${SOURCE_REPO}/yelp/dumb-init.git && cd dumb-init && cc -std=gnu99 -static -s -Wall -Werror -O3 -o dumb-init dumb-init.c

# Build luarocks assets
FROM $BASE_ALT_DEV as luarocks-builder
ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}
RUN apt-get install -y lua5.1-devel \
    && git clone --branch 0.5.1 ${SOURCE_REPO}/starwing/lua-protobuf \
    && cd lua-protobuf/ \
    && luarocks-5.1 make rockspecs/lua-protobuf-scm-1.rockspec
RUN cd / && \
    git clone --branch 7-3 ${SOURCE_REPO}/luarocks-sorces/lua-iconv \
    && cd lua-iconv/ \
    && luarocks-5.1 install lua-iconv-7-3.src.rock

# Build ingress controller, debug tool and pre-stop hook
FROM $BASE_GOLANG_21_BULLSEYE_DEV as controller-builder
ARG CONTROLLER_BRANCH
ENV CONTROLLER_BRANCH=${CONTROLLER_BRANCH}
ARG SOURCE_REPO
ARG GOPROXY
ENV SOURCE_REPO=${SOURCE_REPO} \
    GOPROXY=${GOPROXY}
WORKDIR /src/
COPY patches/lua-info.patch /
COPY patches/makefile.patch /
COPY patches/healthcheck.patch /
COPY patches/metrics-SetSSLExpireTime.patch /
COPY patches/util.patch /
COPY patches/fix-cleanup.patch /
ENV GOARCH=amd64
RUN git clone --branch $CONTROLLER_BRANCH --depth 1 ${SOURCE_REPO}/kubernetes/ingress-nginx.git /src && \
    patch -p1 < /lua-info.patch && \
    patch -p1 < /makefile.patch && \
    patch -p1 < /healthcheck.patch && \
    patch -p1 < /metrics-SetSSLExpireTime.patch && \
    patch -p1 < /util.patch && \
    patch -p1 < /fix-cleanup.patch && \
    make GO111MODULE=on USE_DOCKER=false build

# Build nginx for ingress controller
FROM $BASE_ALT_DEV as nginx-builder
ARG CONTROLLER_BRANCH
ENV CONTROLLER_BRANCH=${CONTROLLER_BRANCH}
ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}
COPY --from=controller-builder /src/images/nginx-1.25/rootfs/ /
COPY rootfs/etc /etc/
COPY patches/nginx-build.patch /
RUN apt-get update && apt-get install -y ninja-build libabseil-cpp-devel build-essential libgrpc-devel  \
    libprotobuf-devel grpc-plugins openssl libpcre2-devel libcares-devel
RUN patch build.sh < nginx-build.patch && /build.sh

# This intermediary image will be used only to copy all the required files to the chroot
# Based on tag "controller-v1.10.1":
# - https://github.com/kubernetes/ingress-nginx/blob/controller-v1.10.1/images/nginx-1.25/rootfs/Dockerfile
# - https://github.com/kubernetes/ingress-nginx/blob/controller-v1.10.1/rootfs/Dockerfile-chroot
# - https://github.com/kubernetes/ingress-nginx/blob/controller-v1.10.1/rootfs/chroot.sh
FROM $BASE_ALT_DEV as chroot

COPY --from=nginx-builder /usr/local /usr/local
COPY --from=nginx-builder /opt /opt
COPY --from=nginx-builder /etc/nginx /etc/nginx

COPY --from=dumb-init-builder /dumb-init/dumb-init /usr/bin/dumb-init

COPY --from=luarocks-builder /usr/lib64/lua/5.1/iconv.so /usr/local/lib/lua/5.1/
COPY --from=luarocks-builder /usr/lib64/lua/5.1/pb.so /usr/local/lib/lua/5.1/
COPY --from=luarocks-builder /usr/share/lua/5.1/protoc.lua /usr/local/share/lua/5.1/

COPY patches/balancer-lua.patch /
COPY patches/nginx-tmpl.patch /
COPY patches/auth-cookie-always.patch /

# copy complete set of libs
COPY --from=base-alt /usr/lib64 /chroot/usr/lib64
COPY --from=base-alt /lib64 /chroot/lib64

RUN ln -s /usr/local/nginx/sbin/nginx /sbin/nginx \
  && adduser -r -U -u 101 -d /usr/local/nginx \
    -s /sbin/nologin -c www-data www-data \
  && bash -eu -c ' \
  writeDirs=( \
  /var/log/nginx \
  /var/lib/nginx/body \
  /var/lib/nginx/fastcgi \
  /var/lib/nginx/proxy \
  /var/lib/nginx/scgi \
  /var/lib/nginx/uwsgi \
  /var/log/audit \
  ); \
  for dir in "${writeDirs[@]}"; do \
  mkdir -p ${dir}; \
  chown -R www-data.www-data ${dir}; \
  done'

# chroot.sh
RUN bash -eu -c ' \
  writeDirs=( \
    /chroot/etc/nginx \
    /chroot/usr/local/nginx \
    /chroot/usr/share \
    /chroot/usr/bin \
    /chroot/etc/ingress-controller \
    /chroot/etc/ingress-controller/ssl \
    /chroot/etc/ingress-controller/auth \
    /chroot/etc/ingress-controller/telemetry \
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
  && mkdir -p /chroot/lib /chroot/lib64 /chroot/proc /chroot/usr /chroot/bin /chroot/dev /chroot/run /chroot/usr/lib64 /chroot/usr/local/modsecurity /chroot/usr/local/share \
  && cp /etc/passwd /etc/group /etc/hosts /chroot/etc/ \
  # Create opentelemetry.toml file as it doesn't present in controller_image
  && touch /chroot/etc/nginx/opentelemetry.toml /chroot/etc/ingress-controller/telemetry/opentelemetry.toml \
  && chown -R www-data.www-data /chroot/etc/nginx/opentelemetry.toml /chroot/etc/ingress-controller/telemetry/opentelemetry.toml \
  && mkdir -p /chroot/etc/nginx/geoip \
  && cp -a /etc/pki /chroot/etc/pki \
  && cp -a /usr/share/ca-certificates /chroot/usr/share/ca-certificates \
  && cp -a /usr/bin/curl /chroot/usr/bin/curl \
  && cp -a /lib64/* /chroot/lib64/ \
  && cp -a /usr/lib64/libcurl* /chroot/usr/lib64/ \
  && cp -a /usr/lib64/libstdc++* /chroot/usr/lib64/ \
  && cp -a /usr/lib64/libbrotli* /chroot/usr/lib64/ \
  && cp -a /usr/lib64/libxml2.so* /chroot/usr/lib64/ \
  && cp -a /usr/lib64/libyajl.so* /chroot/usr/lib64/ \
  && cp -a /usr/lib64/libmaxminddb.* /chroot/usr/lib64/ \
  && cp -a /usr/lib64/libgsasl.so* /chroot/usr/lib64/ \
  && cp -a /usr/lib64/libnfnetlink.so* /chroot/usr/lib64/ \
  && cp -a /usr/lib64/libntlm.so* /chroot/usr/lib64/ \
  && cp -a /usr/lib64/libnetfilter_conntrack.so* /chroot/usr/lib64/ \
  && cp -a /usr/lib64/libnghttp2.so* /chroot/usr/lib64/ \
  && cp -a /usr/lib64/libpsl.so* /chroot/usr/lib64/ \
  && cp -a /usr/lib64/libssh2.so* /chroot/usr/lib64/ \
  && cp -a /usr/lib64/gconv /chroot/usr/lib64/ \
  && cp -a /etc/nginx/* /chroot/etc/nginx/ \
  && cp -a /usr/local/bin /chroot/usr/local/ \
  && cp -a /usr/local/lib /chroot/usr/local/ \
  && cp -a /usr/local/share/lua* /chroot/usr/local/share/ \
  && cp -a /usr/local/lib64 /chroot/usr/local/ \
  && cp -a /usr/local/modsecurity/bin /chroot/usr/local/modsecurity/ \
  && cp -a /usr/local/modsecurity/lib/libmodsecurity.* /chroot/usr/lib64/ \
  && cp -a /usr/local/nginx /chroot/usr/local/

COPY --from=controller-builder --chown=www-data:www-data /src/rootfs/etc /chroot/etc

RUN patch -p1 < /balancer-lua.patch && patch -p1 < /nginx-tmpl.patch && patch -p1 < /auth-cookie-always.patch

# Final image
# Based on tag "controller-v1.10.1":
# - https://github.com/kubernetes/ingress-nginx/blob/be46124ccc9e8158165f06e3f7d2ebd0cbbb284f/rootfs/Dockerfile-chroot
FROM $BASE_ALT

ENV PATH=$PATH:/usr/local/luajit/bin:/usr/local/nginx/sbin:/usr/local/nginx/bin
ENV LUA_PATH="/usr/local/share/luajit-2.1/?.lua;/usr/local/share/lua/5.1/?.lua;/usr/local/lib/lua/?.lua;;"
ENV LUA_CPATH="/usr/local/lib/lua/?/?.so;/usr/local/lib/lua/?.so;;"

RUN ln -s /usr/local/nginx/sbin/nginx /sbin/nginx \
  && adduser -r -U -u 101 -d /usr/local/nginx \
    -s /sbin/nologin -c www-data www-data

COPY --from=chroot /chroot /chroot

COPY --from=controller-builder /src/rootfs/bin/amd64/dbg /
COPY --from=controller-builder /src/rootfs/bin/amd64/nginx-ingress-controller  /
COPY --from=controller-builder /src/rootfs/bin/amd64/wait-shutdown /
COPY --chown=www-data:www-data nginx-chroot-wrapper.sh /usr/bin/nginx
COPY --chown=www-data:www-data curl-chroot-wrapper.sh /usr/bin/curl

COPY --from=dumb-init-builder /dumb-init/dumb-init /usr/bin/dumb-init

RUN  chmod 1777 /tmp \
  && setcap     cap_sys_chroot,cap_net_bind_service=+ep /nginx-ingress-controller \
  && setcap -v cap_sys_chroot,cap_net_bind_service=+ep /nginx-ingress-controller \
  && setcap    cap_sys_chroot,cap_net_bind_service=+ep /usr/bin/unshare \
  && setcap -v cap_sys_chroot,cap_net_bind_service=+ep /usr/bin/unshare \
  && setcap    cap_net_bind_service=+ep /chroot/usr/local/nginx/sbin/nginx \
  && setcap -v cap_net_bind_service=+ep /chroot/usr/local/nginx/sbin/nginx \
  && setcap    cap_sys_chroot,cap_net_bind_service=+ep /usr/bin/dumb-init \
  && setcap -v cap_sys_chroot,cap_net_bind_service=+ep /usr/bin/dumb-init \
  && rm -rf /etc/pki \
  && ln -sf /chroot/etc/pki /etc/pki \
  && ln -sf /chroot/usr/share/ca-certificates /usr/share/ca-certificates \
  && ln -sf /chroot/etc/nginx /etc/nginx \
  && ln -sf /chroot/tmp/nginx /tmp/nginx \
  && ln -sf /chroot/etc/ingress-controller /etc/ingress-controller \
  && ln -sf /chroot/var/log/nginx /var/log/nginx \
  && touch /chroot/var/log/nginx/access.log \
  && chown www-data:www-data /chroot/var/log/nginx/access.log \
  && echo "" > /chroot/etc/resolv.conf \
  && chown -R www-data.www-data /var/log /chroot/var/log /chroot/etc/resolv.conf \
  && mknod -m 0666 /chroot/dev/null c 1 3 \
  && mknod -m 0666 /chroot/dev/random c 1 8 \
  && mknod -m 0666 /chroot/dev/urandom c 1 9 \
  && mknod -m 0666 /chroot/dev/full c 1 7 \
  && mknod -m 0666 /chroot/dev/ptmx c 5 2 \
  && mknod -m 0666 /chroot/dev/zero c 1 5 \
  && mknod -m 0666 /chroot/dev/tty c 5 0 \
  && echo -e "/lib\n/lib64\n/usr/local/lib\n/usr/local/lib64\n/etc/nginx/modules/otel_ngx_module.so" > /etc/ld.so.conf.d/local.conf \
  && ldconfig \
  # Create ld.so.cache inside chroot
  && cp -a /etc/ld.so.conf* /chroot/etc/ && ldconfig -r /chroot

COPY --chown=www-data:www-data rootfs/etc /chroot/etc/
WORKDIR /
USER www-data
EXPOSE 80 443
ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/nginx-ingress-controller"]
