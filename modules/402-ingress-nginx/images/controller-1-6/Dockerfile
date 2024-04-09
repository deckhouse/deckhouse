ARG CONTROLLER_BRANCH=controller-v1.6.4
ARG BASE_ALPINE
ARG BASE_ALT_DEV
# controller artifact
ARG BASE_GOLANG_19_BUSTER
FROM $BASE_GOLANG_19_BUSTER as artifact
ARG CONTROLLER_BRANCH
ENV CONTROLLER_BRANCH=${CONTROLLER_BRANCH}
ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}
WORKDIR /src/
COPY patches/lua-info.patch /
COPY patches/makefile.patch /
COPY patches/healthcheck.patch /
COPY patches/metrics-SetSSLExpireTime.patch /
COPY patches/endpointslice-missing-conditions.patch /
COPY patches/fix-cleanup.patch /
ENV GOARCH=amd64
RUN apt-get update && apt-get install -y --no-install-recommends git mercurial patch && \
    git clone --branch $CONTROLLER_BRANCH --depth 1 ${SOURCE_REPO}/kubernetes/ingress-nginx.git /src && \
    patch -p1 < /lua-info.patch && \
    patch -p1 < /makefile.patch && \
    patch -p1 < /healthcheck.patch && \
    patch -p1 < /metrics-SetSSLExpireTime.patch && \
    patch -p1 < /endpointslice-missing-conditions.patch && \
    patch -p1 < /fix-cleanup.patch && \
    make GO111MODULE=on USE_DOCKER=false build

# luarocks assets for luajit artifact
FROM $BASE_ALT_DEV as luarocks_artifact
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
FROM $BASE_ALT_DEV as nginx_artifact
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

RUN git clone --branch 8.45 --depth 1 ${SOURCE_REPO}/pcre/pcre.git && \
    # build pcre library with jit support due to lack of jit support in standard alt pcre library
    cd pcre && \
    ./configure --prefix=/usr/local/pcre --enable-utf8 --enable-unicode-properties --enable-pcre8 --enable-pcre16 --enable-pcre32 --with-match-limit-recursion=8192 --enable-jit && \
    make && \
    make install && \
    cd / && \
    patch build.sh < nginx-build.patch && \
    /build.sh

# Final image
FROM $BASE_ALT_DEV
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
COPY --from=luarocks_artifact /usr/lib64/lua/5.1/iconv.so /usr/local/lib/lua/5.1/
COPY --from=luarocks_artifact /usr/lib64/lua/5.1/pb.so /usr/local/lib/lua/5.1/
COPY --from=luarocks_artifact /usr/share/lua/5.1/protoc.lua /usr/local/share/lua/5.1/
COPY --from=dumb_init_artifact /dumb-init/dumb-init /usr/bin/dumb-init
COPY patches/balancer-lua.patch /
COPY patches/nginx-tmpl.patch /
COPY patches/auth-cookie-always.patch /
COPY rootfs /

RUN apt-get update \
  && apt-get install -y openssl pcre zlib libGeoIP curl ca-certificates patch yajl liblmdb libxml2 libmaxminddb libyaml-cpp0 nano tzdata diffutils libcap glibc-gconv-modules libbrotlienc \
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
  done'

# replace pcre with version with jit support
RUN rm -f /lib64/libpcre*
COPY --from=nginx_artifact /usr/local/pcre/lib/libpcre.so.1.2.13 /usr/local/pcre/lib/libpcre16.so.0.2.13 /usr/local/pcre/lib/libpcre32.so.0.0.13  /usr/local/pcre/lib/libpcrecpp.so.0.0.2 /usr/local/pcre/lib/libpcreposix.so.0.0.7 /lib64/

RUN echo "/lib:/usr/lib:/usr/local/lib:/modules_mount/etc/nginx/modules/otel" > /etc/ld-musl-x86_64.path \
  && chmod 1777 /tmp \
  && setcap    cap_net_bind_service=+ep /nginx-ingress-controller \
  && setcap -v cap_net_bind_service=+ep /nginx-ingress-controller \
  && setcap    cap_net_bind_service=+ep /usr/local/nginx/sbin/nginx \
  && setcap -v cap_net_bind_service=+ep /usr/local/nginx/sbin/nginx \
  && setcap    cap_net_bind_service=+ep /usr/bin/dumb-init \
  && setcap -v cap_net_bind_service=+ep /usr/bin/dumb-init
# Create symlinks to redirect nginx logs to stdout and stderr docker log collector
RUN ln -sf /dev/stdout /var/log/nginx/access.log \
  && ln -sf /dev/stderr /var/log/nginx/error.log \
  && ln -sf /usr/local/nginx/sbin/nginx /usr/bin/nginx \
  && echo -e "/usr/local/lib\n/usr/local/lib64" > /etc/ld.so.conf.d/local.conf \
  && ldconfig \
  # fix simlink to proper pcre jit version
  && ln -s libpcre.so.1.2.13 /lib64/libpcre.so.3

RUN cd / && patch -p1 < /balancer-lua.patch \
  && patch -p1 < /nginx-tmpl.patch \
  && patch -p1 < /auth-cookie-always.patch \
  && rm -rf /*.patch
WORKDIR /
USER www-data
EXPOSE 80 443
ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/nginx-ingress-controller"]
