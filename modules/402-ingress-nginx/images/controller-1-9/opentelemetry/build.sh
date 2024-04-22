#!/bin/bash

# Copyright 2021 The Kubernetes Authors.
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

set -o errexit
set -o nounset
set -o pipefail

export SOURCE_REPO="${SOURCE_REPO}"
# Check for recent changes: ${SOURCE_REPO}/open-telemetry/opentelemetry-cpp/compare/v1.2.0...main
export OPENTELEMETRY_CPP_VERSION=${OPENTELEMETRY_CPP_VERSION:="1.2.0"}
export INSTALL_DIR=/opt/third_party/install
# improve compilation times
CORES=$(($(grep -c ^processor /proc/cpuinfo) - 1))

rm -rf \
   /var/cache/debconf/* \
   /var/lib/apt/lists/* \
   /var/log/* \
   /tmp/* \
   /var/tmp/*

export BUILD_PATH=/tmp/build
mkdir --verbose -p "$BUILD_PATH"

Help()
{
   # Display Help
   echo "Add description of the script functions here."
   echo
   echo "Syntax: scriptTemplate [-h|o|n|]"
   echo "options:"
   echo "h     Print Help."
   echo "o     OpenTelemetry git tag"
   echo "n     install nginx"
   echo
}

install_otel()
{
  cd ${BUILD_PATH}
  export LD_LIBRARY_PATH="${LD_LIBRARY_PATH:+LD_LIBRARY_PATH:}${INSTALL_DIR}/lib:/usr/local"
  export PATH="${PATH}:${INSTALL_DIR}/bin"
  git clone -j ${CORES} --depth=1 -b \
    ${OPENTELEMETRY_CPP_VERSION} ${SOURCE_REPO}/open-telemetry/opentelemetry-cpp.git opentelemetry-cpp-${OPENTELEMETRY_CPP_VERSION}
  cd "opentelemetry-cpp-${OPENTELEMETRY_CPP_VERSION}"
  mkdir -p .build
  cd .build

  cmake -DCMAKE_BUILD_TYPE=Release \
        -G Ninja \
        -DCMAKE_CXX_STANDARD=17 \
        -DCMAKE_POSITION_INDEPENDENT_CODE=TRUE  \
        -DWITH_ZIPKIN=OFF \
        -DCMAKE_INSTALL_PREFIX=${INSTALL_DIR} \
        -DBUILD_TESTING=OFF \
        -DWITH_BENCHMARK=OFF \
        -DWITH_FUNC_TESTS=OFF \
        -DBUILD_SHARED_LIBS=OFF \
        -DWITH_OTLP_GRPC=ON \
        -DWITH_OTLP_HTTP=OFF \
        -DWITH_ABSEIL=ON \
        -DWITH_EXAMPLES=OFF \
        -DWITH_NO_DEPRECATED_CODE=ON \
        ..
  cmake --build . -j ${CORES} --target install
}

install_nginx()
{
  export NGINX_VERSION=1.21.6

  # Check for recent changes: ${SOURCE_REPO}/open-telemetry/opentelemetry-cpp-contrib/compare/2656a4...main
  # export OPENTELEMETRY_CONTRIB_COMMIT=aaa51e2297bcb34297f3c7aa44fa790497d2f7f3

  mkdir -p /etc/nginx
  cd "$BUILD_PATH"

  git clone --recurse-submodules ${SOURCE_REPO}/open-telemetry/opentelemetry-cpp-contrib.git -b v0.0.1-flant \
    opentelemetry-cpp-contrib
  cd ${BUILD_PATH}/opentelemetry-cpp-contrib
  cd ${BUILD_PATH}/opentelemetry-cpp-contrib/instrumentation/nginx
  mkdir -p build
  cd build
  cmake -DCMAKE_BUILD_TYPE=Release \
        -G Ninja \
        -DCMAKE_CXX_STANDARD=17 \
        -DCMAKE_INSTALL_PREFIX=${INSTALL_DIR} \
        -DBUILD_SHARED_LIBS=ON \
        -DNGINX_VERSION=${NGINX_VERSION} \
        ..
  cmake --build . -j ${CORES} --target install

  mkdir -p /etc/nginx/modules
  cp ${INSTALL_DIR}/otel_ngx_module.so /etc/nginx/modules/otel_ngx_module.so
}

while getopts ":h:o:n" option; do
   case $option in
    h) # display Help
         Help
         exit;;
    o) # install OpenTelemetry tag
        OPENTELEMETRY_CPP_VERSION=${OPTARG}
        install_otel
        exit;;
    n) # install nginx
        install_nginx
        exit;;
    \?)
        Help
        exit;;
   esac
done
