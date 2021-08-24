ARG BASE_DEBIAN
# controller artifact
ARG BASE_GOLANG_BUSTER
FROM $BASE_GOLANG_BUSTER as artifact
WORKDIR /src/
COPY patches/lua-info.patch /
COPY patches/reason.patch /
COPY patches/omit-helm-secrets.patch /
COPY patches/pod-ip.patch /
ENV GOARCH=amd64
RUN apt-get update && apt-get install -y --no-install-recommends git mercurial patch && \
    git clone --branch nginx-0.26.1 --depth 1 https://github.com/kubernetes/ingress-nginx.git /src && \
    patch -p1 < /lua-info.patch && \
    patch -p1 < /reason.patch && \
    patch -p1 < /pod-ip.patch && \
    patch -p1 < /omit-helm-secrets.patch && \
    make GO111MODULE=on build

# luarocks assets for luajit artifact
FROM quay.io/kubernetes-ingress-controller/nginx-ingress-controller:0.26.1@sha256:d0b22f715fcea5598ef7f869d308b55289a3daaa12922fa52a1abf17703c88e7 as controller_lua
USER root
RUN apt-get update \
  && apt-get install -y --no-install-recommends patch gcc build-essential \
  && luarocks install lua-protobuf 0.3.2-0 \
  && luarocks install lua-iconv 7-3

# IngressNginxController docker image
FROM quay.io/kubernetes-ingress-controller/nginx-ingress-controller:0.26.1@sha256:d0b22f715fcea5598ef7f869d308b55289a3daaa12922fa52a1abf17703c88e7 as controller_0_26_1

# Final image
FROM $BASE_DEBIAN
# Based on https://github.com/kubernetes/ingress-nginx/blob/nginx-0.26.1/images/nginx/rootfs/Dockerfile
# Based on https://github.com/kubernetes/ingress-nginx/blob/nginx-0.26.1/rootfs/Dockerfile

ENV PATH=$PATH:/usr/local/openresty/luajit/bin:/usr/local/openresty/nginx/sbin:/usr/local/openresty/bin
# Add LuaRocks paths
# see https://github.com/openresty/docker-openresty/blob/de05cd72594498b83e3a97e2f632da6aa75ec01d/bionic/Dockerfile#L168
ENV LUA_PATH="/usr/local/openresty/site/lualib/?.ljbc;/usr/local/openresty/site/lualib/?/init.ljbc;/usr/local/openresty/lualib/?.ljbc;/usr/local/openresty/lualib/?/init.ljbc;/usr/local/openresty/site/lualib/?.lua;/usr/local/openresty/site/lualib/?/init.lua;/usr/local/openresty/lualib/?.lua;/usr/local/openresty/lualib/?/init.lua;./?.lua;/usr/local/openresty/luajit/share/luajit-2.1.0-beta3/?.lua;/usr/local/share/lua/5.1/?.lua;/usr/local/share/lua/5.1/?/init.lua;/usr/local/openresty/luajit/share/lua/5.1/?.lua;/usr/local/openresty/luajit/share/lua/5.1/?/init.lua;/usr/local/lib/lua/?.lua"
ENV LUA_CPATH="/usr/local/openresty/site/lualib/?.so;/usr/local/openresty/lualib/?.so;./?.so;/usr/local/lib/lua/5.1/?.so;/usr/local/openresty/luajit/lib/lua/5.1/?.so;/usr/local/lib/lua/5.1/loadall.so;/usr/local/openresty/luajit/lib/lua/5.1/?.so"

COPY --from=controller_0_26_1 /usr/local /usr/local
COPY --from=controller_0_26_1 /opt /opt
COPY --from=controller_0_26_1 --chown=www-data:www-data /etc /etc
COPY --from=controller_0_26_1 --chown=www-data:www-data /ingress-controller /ingress-controller

COPY --from=controller_0_26_1 --chown=www-data:www-data /dbg /dbg
COPY --from=controller_0_26_1 --chown=www-data:www-data /nginx-ingress-controller /nginx-ingress-controller
COPY --from=controller_0_26_1 --chown=www-data:www-data /wait-shutdown /wait-shutdown

COPY --from=artifact /src/bin/amd64/nginx-ingress-controller /src/bin/amd64/dbg /
COPY --from=controller_lua /usr/local/openresty/luajit /usr/local/openresty/luajit
COPY patches/balancer-lua.patch /
COPY patches/nginx-tpl.patch /

COPY rootfs /

RUN clean-install \
    bash \
    curl \
    ca-certificates \
    unzip \
    git \
    openssh-client \
    dumb-init \
    libgeoip1 \
    diffutils \
    libcap2-bin \
    patch\
 && cp /usr/local/openresty/nginx/conf/mime.types /etc/nginx/mime.types \
 && cp /usr/local/openresty/nginx/conf/fastcgi_params /etc/nginx/fastcgi_params \
 && ln -s /usr/local/openresty/nginx/modules /etc/nginx/modules \
 && mkdir /var/log/nginx \
# Fix permission during the build to avoid issues at runtime
# with volumes (custom templates)
 && bash -eu -c ' \
  writeDirs=( \
    /etc/ingress-controller/ssl \
    /etc/ingress-controller/auth \
    /var/log \
    /var/log/nginx \
    /tmp \
  ); \
  for dir in "${writeDirs[@]}"; do \
    mkdir -p ${dir}; \
    chown -R www-data.www-data ${dir}; \
  done' \
  && setcap    cap_net_bind_service=+ep /nginx-ingress-controller \
  && setcap -v cap_net_bind_service=+ep /nginx-ingress-controller \
  && setcap    cap_net_bind_service=+ep /usr/local/openresty/nginx/sbin/nginx \
  && setcap -v cap_net_bind_service=+ep /usr/local/openresty/nginx/sbin/nginx \
  && ln -sf /dev/stdout /usr/local/openresty/nginx/logs/access.log \
  && ln -sf /dev/stderr /usr/local/openresty/nginx/logs/error.log \
  && ln -s /usr/local/openresty/nginx/logs/* /var/log/nginx \
  && cd / \
  && patch -p1 < /balancer-lua.patch \
  && patch -p1 < /nginx-tpl.patch

WORKDIR  /etc/nginx
USER www-data
EXPOSE 80 443
ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/nginx-ingress-controller"]

