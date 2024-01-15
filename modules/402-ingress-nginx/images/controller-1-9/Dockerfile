ARG BASE_ALT_P101
ARG BASE_GOLANG_21_BULLSEYE
ARG BASE_DISTROLESS
ARG CONTROLLER_BRANCH=controller-v1.9.5


# Build dumb-init binary
FROM $BASE_ALT_P101 as dumb-init-builder
ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}
RUN apt-get update && apt-get install -y git gcc glibc-devel-static make
RUN git clone --branch v1.2.5 --depth 1 ${SOURCE_REPO}/yelp/dumb-init.git && cd dumb-init && make


# Build luarocks assets
FROM $BASE_ALT_P101 as luarocks-builder
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


# Build ingress controller, debug tool and pre-stop hook
FROM $BASE_GOLANG_21_BULLSEYE as controller-builder
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


# Build nginx for ingress controller
FROM $BASE_ALT_P101 as nginx-builder
ARG CONTROLLER_BRANCH
ENV CONTROLLER_BRANCH=${CONTROLLER_BRANCH}
ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}
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
COPY --from=controller-builder /src/images/nginx/rootfs/ /
COPY rootfs/etc /
COPY patches/nginx-build.patch /
RUN patch build.sh < nginx-build.patch
RUN /build.sh


# This intermediary image will be used only to copy all the required files to the chroot
# Based on tag "controller-v1.9.5":
# - https://github.com/kubernetes/ingress-nginx/blob/be46124ccc9e8158165f06e3f7d2ebd0cbbb284f/images/nginx/rootfs/Dockerfile
# - https://github.com/kubernetes/ingress-nginx/blob/be46124ccc9e8158165f06e3f7d2ebd0cbbb284f/rootfs/Dockerfile-chroot
# - https://github.com/kubernetes/ingress-nginx/blob/be46124ccc9e8158165f06e3f7d2ebd0cbbb284f/rootfs/chroot.sh
FROM $BASE_ALT_P101 as chroot

ENV PATH=$PATH:/usr/local/luajit/bin:/usr/local/nginx/sbin:/usr/local/nginx/bin
ENV LUA_PATH="/usr/local/share/luajit-2.1.0-beta3/?.lua;/usr/local/share/lua/5.1/?.lua;/usr/local/lib/lua/?.lua;;"
ENV LUA_CPATH="/usr/local/lib/lua/?/?.so;/usr/local/lib/lua/?.so;;"

COPY --from=nginx-builder /usr/local /usr/local
COPY --from=nginx-builder /opt /opt
COPY --from=nginx-builder /etc/nginx /etc/nginx

COPY --from=dumb-init-builder /dumb-init/dumb-init /usr/bin/dumb-init

COPY --from=luarocks-builder /usr/lib64/lua/5.1/iconv.so /usr/local/lib/lua/5.1/
COPY --from=luarocks-builder /usr/lib64/lua/5.1/pb.so /usr/local/lib/lua/5.1/
COPY --from=luarocks-builder /usr/share/lua/5.1/protoc.lua /usr/local/share/lua/5.1/

RUN apt-get update \
  && apt-get install -y openssl pcre zlib libGeoIP ca-certificates patch yajl liblmdb libxml2 libmaxminddb libyaml-cpp0 tzdata \
  && ln -s /usr/local/nginx/sbin/nginx /sbin/nginx \
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
    /chroot/usr/local/ \
    /chroot/etc/ingress-controller \
    /chroot/etc/ingress-controller/ssl \
    /chroot/etc/ingress-controller/auth \
    /chroot/etc/ingress-controller/telemetry \
    /chroot/etc/ingress-controller/geoip \
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
  && mkdir -p /chroot/lib /chroot/lib64 /chroot/proc /chroot/usr /chroot/bin /chroot/dev /chroot/run \
  && cp /etc/passwd /etc/group /etc/hosts /chroot/etc/ \
  && cp -a /usr/* /chroot/usr/ \
  # Create opentelemetry.toml file as it doesn't present in controller_image
  && touch /chroot/etc/nginx/opentelemetry.toml /chroot/etc/ingress-controller/telemetry/opentelemetry.toml \
  && chown -R www-data.www-data /chroot/etc/nginx/opentelemetry.toml \
  && mkdir -p /chroot/etc/nginx/geoip \
  && cp -a /etc/nginx/* /chroot/etc/nginx/ \
  && cp -a /lib/* /chroot/lib/ \
  && cp -a /lib64/* /chroot/lib64/


# Final image
# Based on tag "controller-v1.9.5":
# - https://github.com/kubernetes/ingress-nginx/blob/be46124ccc9e8158165f06e3f7d2ebd0cbbb284f/rootfs/Dockerfile-chroot
FROM $BASE_ALT_P101

ENV PATH=$PATH:/usr/local/luajit/bin:/usr/local/nginx/sbin:/usr/local/nginx/bin
ENV LUA_PATH="/usr/local/share/luajit-2.1.0-beta3/?.lua;/usr/local/share/lua/5.1/?.lua;/usr/local/lib/lua/?.lua;;"
ENV LUA_CPATH="/usr/local/lib/lua/?/?.so;/usr/local/lib/lua/?.so;;"

ENV LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/usr/local/lib:/usr/local/lib64/:/modules_mount/etc/nginx/modules/otel

RUN apt-get update \
  && apt-get install -y openssl curl ca-certificates nano tzdata diffutils util-linux patch \
  && ln -s /usr/local/nginx/sbin/nginx /sbin/nginx \
  && adduser -r -U -u 101 -d /usr/local/nginx \
    -s /sbin/nologin -c www-data www-data

COPY --from=chroot /chroot /chroot

COPY --from=controller-builder --chown=www-data:www-data /src/rootfs/etc /chroot/etc

COPY --from=controller-builder /src/rootfs/bin/amd64/dbg /
COPY --from=controller-builder /src/rootfs/bin/amd64/nginx-ingress-controller  /
COPY --from=controller-builder /src/rootfs/bin/amd64/wait-shutdown /
COPY --chown=www-data:www-data nginx-chroot-wrapper.sh /usr/bin/nginx

COPY --from=dumb-init-builder /dumb-init/dumb-init /usr/bin/dumb-init

COPY patches/balancer-lua.patch /
COPY patches/nginx-tmpl.patch /
COPY patches/auth-cookie-always.patch /

RUN setcap     cap_sys_chroot,cap_net_bind_service=+ep /nginx-ingress-controller \
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
  && mkdir -p /chroot/modules_mount \
  && mkdir -p modules_mount \
  && ln -s /modules_mount /chroot/modules_mount \
  && echo -e "/usr/local/lib\n/usr/local/lib64\n/modules_mount/etc/nginx/modules/otel" > /etc/ld.so.conf.d/local.conf \
  && ldconfig \
  # Create ld.so.cache inside chroot
  && cp -a /etc/ld.so.conf* /chroot/etc/ && ldconfig -r /chroot

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
