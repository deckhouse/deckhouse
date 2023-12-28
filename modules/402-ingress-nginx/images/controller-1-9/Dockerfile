ARG CONTROLLER_BRANCH=controller-v1.9.4
ARG BASE_ALPINE
ARG BASE_ALT_P101
# controller artifact
ARG BASE_GOLANG_21_BULLSEYE
FROM $BASE_GOLANG_21_BULLSEYE as artifact
ARG CONTROLLER_BRANCH
ENV CONTROLLER_BRANCH=${CONTROLLER_BRANCH}
ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}
WORKDIR /src/
COPY patches/lua-info.patch /
COPY patches/makefile.patch /
COPY patches/healthcheck.patch /
COPY patches/metrics-SetSSLExpireTime.patch /
COPY patches/util.patch /
COPY patches/fix-cleanup.patch /
ENV GOARCH=amd64
RUN apt-get update && apt-get install -y --no-install-recommends git mercurial patch && \
    git clone --branch $CONTROLLER_BRANCH --depth 1 ${SOURCE_REPO}/kubernetes/ingress-nginx.git /src && \
    patch -p1 < /lua-info.patch && \
    patch -p1 < /makefile.patch && \
    patch -p1 < /healthcheck.patch && \
    patch -p1 < /metrics-SetSSLExpireTime.patch && \
    patch -p1 < /util.patch && \
    patch -p1 < /fix-cleanup.patch && \
    make GO111MODULE=on USE_DOCKER=false build

# luarocks assets for luajit artifact
FROM $BASE_ALT_P101 as luarocks_artifact
ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}
RUN apt-get update && apt-get install -y gcc gcc-c++ git lua5.1-luarocks lua5.1-devel
RUN git clone --branch 0.4.1 ${SOURCE_REPO}/starwing/lua-protobuf \
    && cd lua-protobuf/ \
    && luarocks-5.1 make rockspecs/lua-protobuf-scm-1.rockspec
RUN cd / && \
    git clone --branch 7-3 ${SOURCE_REPO}/luarocks-sorces/lua-iconv \
    && cd lua-iconv/ \
    && luarocks-5.1 install lua-iconv-7-3.src.rock


# dumb-init artifact
FROM $BASE_ALPINE as dumb_init_artifact
ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}
RUN apk update && apk add git gcc g++ bash make
RUN git clone --branch v1.2.5 ${SOURCE_REPO}/yelp/dumb-init.git && cd dumb-init && make

#nginx for controller
FROM $BASE_ALT_P101 as nginx_artifact
ARG CONTROLLER_BRANCH
ENV CONTROLLER_BRANCH=${CONTROLLER_BRANCH}
ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}
# install required packages to build
RUN apt-get update && apt-get install -y \
  gcc \
  clang \
  glibc-devel \
  make \
  automake \
  openssl-devel \
  pcre-devel \
  zlib-devel \
  kernel-headers-common \
  libxslt-devel \
  libgd3-devel \
  perl-devel \
  libedit-devel \
  mercurial \
  findutils \
  curl \
  ca-certificates \
  patch \
  libaio-devel \
  openssl \
  cmake \
  util-linux \
  wget \
  curl-devel \
  libprotobuf \
  liblmdb-devel \
  git gcc-c++ flex bison doxygen libyajl-devel libtool autoconf libxml2 libxml2-devel \
  python3 \
  bc \
  libmaxminddb-devel \
  unzip \
  dos2unix \
  libyaml-cpp0 \
  coreutils \
  libstdc++-devel \
  pkg-config \
  libstdc++-devel-static \
  libGeoIP-devel \
  lua-devel \
  libbrotli-devel \
  patch

COPY --from=artifact /src/images/nginx/rootfs/ /
COPY patches/nginx-build.patch /

RUN patch build.sh < nginx-build.patch

RUN /build.sh

# Final image
FROM $BASE_ALT_P101
# Based on https://github.com/kubernetes/ingress-nginx/blob/controller-v1.0.4/images/nginx/rootfs/Dockerfile
# Based on https://github.com/kubernetes/ingress-nginx/blob/controller-v1.0.4/rootfs/Dockerfile
ENV PATH=$PATH:/usr/local/luajit/bin:/usr/local/nginx/sbin:/usr/local/nginx/bin

ENV LUA_PATH="/usr/local/share/luajit-2.1.0-beta3/?.lua;/usr/local/share/lua/5.1/?.lua;/usr/local/lib/lua/?.lua;;"
ENV LUA_CPATH="/usr/local/lib/lua/?/?.so;/usr/local/lib/lua/?.so;;"
# Fix for openrestry luarocks paths
ENV PATH=$PATH:/usr/local/openresty/luajit/bin:/usr/local/openresty/nginx/sbin:/usr/local/openresty/bin
ENV LUA_PATH=$LUA_PATH;/usr/local/openresty/site/lualib/?.ljbc;/usr/local/openresty/site/lualib/?/init.ljbc;/usr/local/openresty/lualib/?.ljbc;/usr/local/openresty/lualib/?/init.ljbc;/usr/local/openresty/site/lualib/?.lua;/usr/local/openresty/site/lualib/?/init.lua;/usr/local/openresty/lualib/?.lua;/usr/local/openresty/lualib/?/init.lua;./?.lua;/usr/local/openresty/luajit/share/luajit-2.1.0-beta3/?.lua;/usr/local/share/lua/5.1/?.lua;/usr/local/share/lua/5.1/?/init.lua;/usr/local/openresty/luajit/share/lua/5.1/?.lua;/usr/local/openresty/luajit/share/lua/5.1/?/init.lua;/usr/local/lib/lua/?.lua
ENV LUA_CPATH=$LUA_CPATH;/usr/local/openresty/site/lualib/?.so;/usr/local/openresty/lualib/?.so;./?.so;/usr/local/lib/lua/5.1/?.so;/usr/local/openresty/luajit/lib/lua/5.1/?.so;/usr/local/lib/lua/5.1/loadall.so;/usr/local/openresty/luajit/lib/lua/5.1/?.so

ENV LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/usr/local/lib:/usr/local/lib64/

RUN adduser -r -U -u 101 -d /usr/local/nginx \
    -s /sbin/nologin -c www-data www-data

COPY --from=nginx_artifact /usr/local /usr/local
COPY --from=nginx_artifact /opt /opt
COPY --from=nginx_artifact --chown=www-data:www-data /etc /etc
COPY --from=artifact --chown=www-data:www-data /src/rootfs /
COPY --from=artifact /src/rootfs/bin/amd64/nginx-ingress-controller /src/rootfs/bin/amd64/dbg /
COPY --from=artifact /src/rootfs/bin/amd64/wait-shutdown /
COPY --from=artifact --chown=www-data:www-data /src/rootfs/etc/nginx/template /etc/nginx/template
COPY --from=luarocks_artifact /usr/lib64/lua/5.1/iconv.so /usr/local/lib/lua/5.1/
COPY --from=luarocks_artifact /usr/lib64/lua/5.1/pb.so /usr/local/lib/lua/5.1/
COPY --from=luarocks_artifact /usr/share/lua/5.1/protoc.lua /usr/local/share/lua/5.1/
COPY --from=dumb_init_artifact /dumb-init/dumb-init /usr/bin/dumb-init
COPY patches/balancer-lua.patch /
COPY patches/nginx-tmpl.patch /
COPY patches/auth-cookie-always.patch /
COPY --chown=www-data:www-data nginx-chroot-wrapper.sh /usr/bin/nginx
COPY rootfs /

RUN apt-get update \
  && apt-get install -y openssl pcre zlib libGeoIP curl ca-certificates patch yajl liblmdb libxml2 libmaxminddb libyaml-cpp0 nano tzdata diffutils libcap \
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
  done'

# Create opentelemetry.toml file as it doesn't present in controller_image
RUN touch /etc/nginx/opentelemetry.toml \
  && chown -R www-data.www-data /etc/nginx/opentelemetry.toml \
  && mkdir -p /chroot/lib /chroot/proc /chroot/usr /chroot/bin /chroot/dev /chroot/run /chroot/etc \
  && cp /etc/passwd /etc/group /chroot/etc/ \
  && cp -a /usr/* /chroot/usr/ \
  && mkdir -p /etc/ingress-controller/auth \
  && mkdir -p /etc/ingress-controller/ssl \
  && chown -R www-data:www-data /etc/ingress-controller \
  && chmod -R 0755 /etc/ingress-controller \
  && mkdir -p /etc/nginx/geoip \
  && chown -R www-data:www-data /etc/nginx/ \
  && mv /etc/nginx /chroot/etc/ \
  && mv /etc/ingress-controller /chroot/etc/ 

RUN echo "/lib:/usr/lib:/usr/local/lib:/modules_mount/etc/nginx/modules/otel" > /chroot/etc/ld-musl-x86_64.path \
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
  && ln -s /modules_mount /chroot/modules_mount
RUN echo -e "/usr/local/lib\n/usr/local/lib64" > /etc/ld.so.conf.d/local.conf \
  && ldconfig

RUN cd / \
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
