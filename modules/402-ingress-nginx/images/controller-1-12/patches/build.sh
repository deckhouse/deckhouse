#!/bin/bash

# Copyright 2023 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -x
set -o errexit
set -o nounset
set -o pipefail

export NGINX_VERSION=1.25.5

# Check for recent changes: https://github.com/vision5/ngx_devel_kit/compare/v0.3.3...master
export NDK_VERSION=v0.3.3

# Check for recent changes: https://github.com/openresty/set-misc-nginx-module/compare/v0.33...master
export SETMISC_VERSION=796f5a3e518748eb29a93bd450324e0ad45b704e

# Check for recent changes: https://github.com/openresty/headers-more-nginx-module/compare/v0.37...master
export MORE_HEADERS_VERSION=v0.37

# Check for recent changes: https://github.com/atomx/nginx-http-auth-digest/compare/v1.0.0...atomx:master
export NGINX_DIGEST_AUTH=v1.0.0

# Check for recent changes: https://github.com/yaoweibin/ngx_http_substitutions_filter_module/compare/v0.6.4...master
export NGINX_SUBSTITUTIONS=e12e965ac1837ca709709f9a26f572a54d83430e

# Check for recent changes: https://github.com/SpiderLabs/ModSecurity-nginx/compare/v1.0.3...master
export MODSECURITY_VERSION=v1.0.3

# Check for recent changes: https://github.com/SpiderLabs/ModSecurity/compare/v3.0.14...v3/master
export MODSECURITY_LIB_VERSION=v3.0.14

# Check for recent changes: https://github.com/coreruleset/coreruleset/compare/v4.10.0...main
export OWASP_MODSECURITY_CRS_VERSION=v4.10.0

# Check for recent changes: https://github.com/openresty/lua-nginx-module/compare/v0.10.26``...master
export LUA_NGX_VERSION=v0.10.26

# Check for recent changes: https://github.com/openresty/stream-lua-nginx-module/compare/bea8a0c0de94cede71554f53818ac0267d675d63...master
export LUA_STREAM_NGX_VERSION=bea8a0c0de94cede71554f53818ac0267d675d63

# Check for recent changes: https://github.com/openresty/lua-upstream-nginx-module/compare/8aa93ead98ba2060d4efd594ae33a35d153589bf...master
export LUA_UPSTREAM_VERSION=542be0893543a4e42d89f6dd85372972f5ff2a36

# Check for recent changes: https://github.com/openresty/lua-cjson/compare/2.1.0.13...openresty:master
export LUA_CJSON_VERSION=2.1.0.13

# Check for recent changes: https://github.com/leev/ngx_http_geoip2_module/compare/a607a41a8115fecfc05b5c283c81532a3d605425...master
export GEOIP2_VERSION=a607a41a8115fecfc05b5c283c81532a3d605425

# Check for recent changes: https://github.com/openresty/luajit2/compare/v2.1-20240314...v2.1-agentzh
export LUAJIT_VERSION=v2.1-20240314

# Check for recent changes: https://github.com/openresty/lua-resty-balancer/compare/1cd4363c0a239afe4765ec607dcfbbb4e5900eea...master
export LUA_RESTY_BALANCER=1cd4363c0a239afe4765ec607dcfbbb4e5900eea

# Check for recent changes: https://github.com/openresty/lua-resty-lrucache/compare/99e7578465b40f36f596d099b82eab404f2b42ed...master
export LUA_RESTY_CACHE=99e7578465b40f36f596d099b82eab404f2b42ed

# Check for recent changes: https://github.com/openresty/lua-resty-core/compare/v0.1.27...master
export LUA_RESTY_CORE=v0.1.28

# Check for recent changes: https://github.com/cloudflare/lua-resty-cookie/compare/f418d77082eaef48331302e84330488fdc810ef4...master
export LUA_RESTY_COOKIE_VERSION=f418d77082eaef48331302e84330488fdc810ef4

# Check for recent changes: https://github.com/openresty/lua-resty-dns/compare/8bb53516e2933e61c317db740a9b7c2048847c2f...master
export LUA_RESTY_DNS=8bb53516e2933e61c317db740a9b7c2048847c2f

# Check for recent changes: https://github.com/ledgetech/lua-resty-http/compare/v0.17.1...master
export LUA_RESTY_HTTP=v0.17.1

# Check for recent changes: https://github.com/openresty/lua-resty-lock/compare/v0.09...master
export LUA_RESTY_LOCK=405d0bf4cbfa74d742c6ed3158d442221e6212a9

# Check for recent changes: https://github.com/openresty/lua-resty-upload/compare/v0.11...master
export LUA_RESTY_UPLOAD_VERSION=979372cce011f3176af3c9aff53fd0e992c4bfd3

# Check for recent changes: https://github.com/openresty/lua-resty-string/compare/v0.15...master
export LUA_RESTY_STRING_VERSION=6f1bc21d86daef804df3cc34d6427ef68da26844

# Check for recent changes: https://github.com/openresty/lua-resty-memcached/compare/v0.17...master
export LUA_RESTY_MEMCACHED_VERSION=2f02b68bf65fa2332cce070674a93a69a6c7239b

# Check for recent changes: https://github.com/openresty/lua-resty-redis/compare/v0.30...master
export LUA_RESTY_REDIS_VERSION=8641b9f1b6f75cca50c90cf8ca5c502ad8950aa8

# Check for recent changes: https://github.com/api7/lua-resty-ipmatcher/compare/v0.6.1...master
export LUA_RESTY_IPMATCHER_VERSION=3e93c53eb8c9884efe939ef070486a0e507cc5be

# Check for recent changes: https://github.com/microsoft/mimalloc/compare/v2.1.7...master
export MIMALOC_VERSION=v2.1.7

# Check for recent changes: https://github.com/open-telemetry/opentelemetry-cpp/compare/v1.18.0...main
export OPENTELEMETRY_CPP_VERSION=v1.18.0

# Check for recent changes: https://github.com/open-telemetry/opentelemetry-proto/compare/v1.5.0...main
export OPENTELEMETRY_PROTO_VERSION=v1.5.0

export BUILD_PATH=/tmp/build

ARCH=$(uname -m)

# apk add -X http://dl-cdn.alpinelinux.org/alpine/edge/testing opentelemetry-cpp-dev

mkdir -p /etc/nginx

mkdir --verbose -p "$BUILD_PATH"
cd "$BUILD_PATH"

git clone -b "${CONTROLLER_BRANCH}" "${SOURCE_REPO}/kubernetes/ingress-nginx-deps.git" .

# improve compilation times
CORES=$(($(grep -c ^processor /proc/cpuinfo) - 1))

export MAKEFLAGS=-j${CORES}
export CTEST_BUILD_FLAGS=${MAKEFLAGS}

# Install luajit from openresty fork
export LUAJIT_LIB=/usr/local/lib
export LUA_LIB_DIR="$LUAJIT_LIB/lua"
export LUAJIT_INC=/usr/local/include/luajit-2.1

cd "$BUILD_PATH/luajit2"
make CCDEBUG=-g
make install

ln -s /usr/local/bin/luajit /usr/local/bin/lua
ln -s "$LUAJIT_INC" /usr/local/include/lua

cd "$BUILD_PATH/opentelemetry-cpp"
export CXXFLAGS="-DBENCHMARK_HAS_NO_INLINE_ASSEMBLY"
cmake -B build -G Ninja -Wno-dev \
        -DOTELCPP_PROTO_PATH="${BUILD_PATH}/opentelemetry-proto/" \
        -DCMAKE_INSTALL_PREFIX=/usr \
        -DBUILD_SHARED_LIBS=ON \
        -DBUILD_TESTING="OFF" \
        -DBUILD_W3CTRACECONTEXT_TEST="OFF" \
        -DCMAKE_BUILD_TYPE=None \
        -DWITH_ABSEIL=ON \
        -DWITH_STL=ON \
        -DWITH_EXAMPLES=OFF \
        -DWITH_ZPAGES=OFF \
        -DWITH_OTLP_GRPC=ON \
        -DWITH_OTLP_HTTP=ON \
        -DWITH_ZIPKIN=ON \
        -DWITH_PROMETHEUS=OFF \
        -DWITH_ASYNC_EXPORT_PREVIEW=OFF \
        -DWITH_METRICS_EXEMPLAR_PREVIEW=OFF
      cmake --build build
      cmake --install build

# Git tuning
git config --global --add core.compression -1

# Get Brotli source and deps
cd "$BUILD_PATH"
git clone --depth=100 https://github.com/google/ngx_brotli.git
cd ngx_brotli
# https://github.com/google/ngx_brotli/issues/156
git reset --hard 63ca02abdcf79c9e788d2eedcc388d2335902e52
git submodule init
git submodule update

cd "$BUILD_PATH"
git clone --depth=1 https://github.com/ssdeep-project/ssdeep
cd ssdeep/

./bootstrap
./configure

make
make install

# build modsecurity library
cd "$BUILD_PATH"
git clone -n https://github.com/SpiderLabs/ModSecurity
cd ModSecurity/
git checkout $MODSECURITY_LIB_VERSION
git submodule init
git submodule update

sh build.sh

# https://github.com/SpiderLabs/ModSecurity/issues/1909#issuecomment-465926762
sed -i '115i LUA_CFLAGS="${LUA_CFLAGS} -DWITH_LUA_JIT_2_1"' build/lua.m4
sed -i '117i AC_SUBST(LUA_CFLAGS)' build/lua.m4

./configure \
  --disable-doxygen-doc \
  --disable-doxygen-html \
  --disable-examples

make
make install

mkdir -p /etc/nginx/modsecurity
cp modsecurity.conf-recommended /etc/nginx/modsecurity/modsecurity.conf
cp unicode.mapping /etc/nginx/modsecurity/unicode.mapping

# Replace serial logging with concurrent
sed -i 's|SecAuditLogType Serial|SecAuditLogType Concurrent|g' /etc/nginx/modsecurity/modsecurity.conf

# Concurrent logging implies the log is stored in several files
echo "SecAuditLogStorageDir /var/log/audit/" >> /etc/nginx/modsecurity/modsecurity.conf

# Download owasp modsecurity crs
cd /etc/nginx/

git clone -b $OWASP_MODSECURITY_CRS_VERSION https://github.com/coreruleset/coreruleset
mv coreruleset owasp-modsecurity-crs
cd owasp-modsecurity-crs

mv crs-setup.conf.example crs-setup.conf
mv rules/REQUEST-900-EXCLUSION-RULES-BEFORE-CRS.conf.example rules/REQUEST-900-EXCLUSION-RULES-BEFORE-CRS.conf
mv rules/RESPONSE-999-EXCLUSION-RULES-AFTER-CRS.conf.example rules/RESPONSE-999-EXCLUSION-RULES-AFTER-CRS.conf
cd ..

# OWASP CRS v4 rules
echo "
Include /etc/nginx/owasp-modsecurity-crs/crs-setup.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-900-EXCLUSION-RULES-BEFORE-CRS.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-901-INITIALIZATION.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-905-COMMON-EXCEPTIONS.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-911-METHOD-ENFORCEMENT.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-913-SCANNER-DETECTION.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-920-PROTOCOL-ENFORCEMENT.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-921-PROTOCOL-ATTACK.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-922-MULTIPART-ATTACK.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-930-APPLICATION-ATTACK-LFI.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-931-APPLICATION-ATTACK-RFI.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-932-APPLICATION-ATTACK-RCE.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-933-APPLICATION-ATTACK-PHP.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-934-APPLICATION-ATTACK-GENERIC.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-941-APPLICATION-ATTACK-XSS.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-942-APPLICATION-ATTACK-SQLI.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-943-APPLICATION-ATTACK-SESSION-FIXATION.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-944-APPLICATION-ATTACK-JAVA.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-949-BLOCKING-EVALUATION.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/RESPONSE-950-DATA-LEAKAGES.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/RESPONSE-951-DATA-LEAKAGES-SQL.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/RESPONSE-952-DATA-LEAKAGES-JAVA.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/RESPONSE-953-DATA-LEAKAGES-PHP.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/RESPONSE-954-DATA-LEAKAGES-IIS.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/RESPONSE-955-WEB-SHELLS.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/RESPONSE-959-BLOCKING-EVALUATION.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/RESPONSE-980-CORRELATION.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/RESPONSE-999-EXCLUSION-RULES-AFTER-CRS.conf
" > /etc/nginx/owasp-modsecurity-crs/nginx-modsecurity.conf

# NGINX compiles a small test program to check if an added module works as expected.
#
# ModSecurity-nginx provides 'printf("hello");' as a test, but newer versions of GCC,
# as included in Alpine 3.21, do not allow implicit declaration of function 'printf':
#
#   objs/autotest.c:7:5: error: implicit declaration of function 'printf' [-Wimplicit-function-declaration]
#
# For this reason we replace 'printf("hello");' by 'msc_init();', which is always available.
#
# This fix is taken from a PR, that has been proposed to the ModSecurity-nginx project:
#
#   https://github.com/owasp-modsecurity/ModSecurity-nginx/pull/275
#
sed -i "s/ngx_feature_test='printf(\"hello\");'/ngx_feature_test='msc_init();'/" $BUILD_PATH/ModSecurity-nginx/config

# build nginx
cd "$BUILD_PATH/nginx-$NGINX_VERSION"

# apply nginx patches
for PATCH in `ls /patches`;do
  echo "Patch: $PATCH"
  if [[ "$PATCH" == *.txt ]]; then
    patch -p0 < /patches/$PATCH
  else
    patch -p1 < /patches/$PATCH
  fi
done

WITH_FLAGS="--with-debug \
  --with-compat \
  --with-pcre-jit \
  --with-http_ssl_module \
  --with-http_stub_status_module \
  --with-http_realip_module \
  --with-http_auth_request_module \
  --with-http_addition_module \
  --with-http_gzip_static_module \
  --with-http_sub_module \
  --with-http_v2_module \
  --with-http_v3_module \
  --with-stream \
  --with-stream_ssl_module \
  --with-stream_realip_module \
  --with-stream_ssl_preread_module \
  --with-threads \
  --with-http_secure_link_module \
  --with-http_gunzip_module"

# "Combining -flto with -g is currently experimental and expected to produce unexpected results."
# https://gcc.gnu.org/onlinedocs/gcc/Optimize-Options.html
CC_OPT="-g -O2 -fPIE -fstack-protector-strong \
  -Wformat \
  -Werror=format-security \
  -Wno-deprecated-declarations \
  -fno-strict-aliasing \
  -D_FORTIFY_SOURCE=2 \
  --param=ssp-buffer-size=4 \
  -DTCP_FASTOPEN=23 \
  -fPIC \
  -Wno-cast-function-type"

LD_OPT="-fPIE -fPIC -pie -Wl,-z,relro -Wl,-z,now"

if [[ ${ARCH} != "aarch64" ]]; then
  WITH_FLAGS+=" --with-file-aio"
fi

if [[ ${ARCH} == "x86_64" ]]; then
  CC_OPT+=' -m64 -mtune=generic'
fi

WITH_MODULES=" \
  --add-module=$BUILD_PATH/ngx_devel_kit \
  --add-module=$BUILD_PATH/set-misc-nginx-module \
  --add-module=$BUILD_PATH/headers-more-nginx-module \
  --add-module=$BUILD_PATH/ngx_http_substitutions_filter_module \
  --add-module=$BUILD_PATH/lua-nginx-module \
  --add-module=$BUILD_PATH/stream-lua-nginx-module \
  --add-module=$BUILD_PATH/lua-upstream-nginx-module \
  --add-dynamic-module=$BUILD_PATH/nginx-http-auth-digest \
  --add-dynamic-module=$BUILD_PATH/ModSecurity-nginx \
  --add-dynamic-module=$BUILD_PATH/ngx_http_geoip2_module \
  --add-dynamic-module=$BUILD_PATH/ngx_brotli"

./configure \
  --prefix=/usr/local/nginx \
  --conf-path=/etc/nginx/nginx.conf \
  --modules-path=/etc/nginx/modules \
  --http-log-path=/var/log/nginx/access.log \
  --error-log-path=/var/log/nginx/error.log \
  --lock-path=/var/lock/nginx.lock \
  --pid-path=/run/nginx.pid \
  --http-client-body-temp-path=/var/lib/nginx/body \
  --http-fastcgi-temp-path=/var/lib/nginx/fastcgi \
  --http-proxy-temp-path=/var/lib/nginx/proxy \
  --http-scgi-temp-path=/var/lib/nginx/scgi \
  --http-uwsgi-temp-path=/var/lib/nginx/uwsgi \
  ${WITH_FLAGS} \
  --without-mail_pop3_module \
  --without-mail_smtp_module \
  --without-mail_imap_module \
  --without-http_uwsgi_module \
  --without-http_scgi_module \
  --with-cc-opt="${CC_OPT}" \
  --with-ld-opt="${LD_OPT}" \
  --user=deckhouse \
  --group=deckhouse \
  ${WITH_MODULES}

make
make modules
make install

# Check for recent changes: https://github.com/open-telemetry/opentelemetry-cpp-contrib/compare/8933841f0a7f8737f61404cf0a64acf6b079c8a5...main
export OPENTELEMETRY_CONTRIB_COMMIT=8933841f0a7f8737f61404cf0a64acf6b079c8a5
cd "$BUILD_PATH"

git clone https://github.com/open-telemetry/opentelemetry-cpp-contrib.git opentelemetry-cpp-contrib-${OPENTELEMETRY_CONTRIB_COMMIT}

cd ${BUILD_PATH}/opentelemetry-cpp-contrib-${OPENTELEMETRY_CONTRIB_COMMIT}
git reset --hard ${OPENTELEMETRY_CONTRIB_COMMIT}

export OTEL_TEMP_INSTALL=/tmp/otel
mkdir -p ${OTEL_TEMP_INSTALL}

cd ${BUILD_PATH}/opentelemetry-cpp-contrib-${OPENTELEMETRY_CONTRIB_COMMIT}/instrumentation/nginx
# remove downloading nginx from cmake and clone it manually
# by default cmake download nginx from internet
# and change cmake NGINX_DIR
sed -e "35d" nginx.cmake | sed -e "35i set(NGINX_DIR \"${BUILD_PATH}/opentelemetry-cpp-contrib-${OPENTELEMETRY_CONTRIB_COMMIT}/instrumentation/nginx/build/nginx/\")" | sed -e "27d" | sed -e "27i \ \ SOURCE_DIR nginx" > nginx.cmake.tmp
rm -f nginx.cmake
mv nginx.cmake.tmp nginx.cmake
cat nginx.cmake
mkdir -p build
cp -R "${BUILD_PATH}/nginx-$NGINX_VERSION" build/nginx
cd build
cmake -DCMAKE_BUILD_TYPE=Release \
        -G Ninja \
        -DCMAKE_CXX_STANDARD=17 \
        -DCMAKE_INSTALL_PREFIX=${OTEL_TEMP_INSTALL} \
        -DBUILD_SHARED_LIBS=ON \
        -DNGINX_VERSION=${NGINX_VERSION} \
        ..
cmake --build . -j ${CORES} --target install

mkdir -p /etc/nginx/modules
cp ${OTEL_TEMP_INSTALL}/otel_ngx_module.so /etc/nginx/modules/otel_ngx_module.so


cd "$BUILD_PATH/lua-resty-core"
make install

cd "$BUILD_PATH/lua-resty-balancer"
make all
make install

export LUA_INCLUDE_DIR=/usr/local/include/luajit-2.1
ln -s $LUA_INCLUDE_DIR /usr/include/lua5.1

cd "$BUILD_PATH/lua-cjson"
make all
make install

cd "$BUILD_PATH/lua-resty-cookie"
make all
make install

cd "$BUILD_PATH/lua-resty-lrucache"
make install

cd "$BUILD_PATH/lua-resty-dns"
make install

cd "$BUILD_PATH/lua-resty-lock"
make install

# required for OCSP verification
cd "$BUILD_PATH/lua-resty-http"
make install

cd "$BUILD_PATH/lua-resty-upload"
make install

cd "$BUILD_PATH/lua-resty-string"
make install

cd "$BUILD_PATH/lua-resty-memcached"
make install

cd "$BUILD_PATH/lua-resty-redis"
make install

cd "$BUILD_PATH/lua-resty-ipmatcher"
INST_LUADIR=/usr/local/lib/lua make install

cd "$BUILD_PATH/mimalloc"
mkdir -p out/release
cd out/release

cmake ../..

make
make install

# update image permissions
writeDirs=( \
  /etc/nginx \
  /usr/local/nginx \
  /opt/modsecurity/var/log \
  /opt/modsecurity/var/upload \
  /opt/modsecurity/var/audit \
  /var/log/audit \
  /var/log/nginx \
);

for dir in "${writeDirs[@]}"; do
  mkdir -p ${dir};
  chown -R deckhouse:deckhouse ${dir};
done

rm -rf /etc/nginx/owasp-modsecurity-crs/.git
rm -rf /etc/nginx/owasp-modsecurity-crs/tests

# remove .a files
find /usr/local -name "*.a" -print | xargs /bin/rm
