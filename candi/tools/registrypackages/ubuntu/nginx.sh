# Copyright 2021 Flant JSC
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

#!/usr/bin/env bash

set -Eeo pipefail

if [[ -z ${REGISTRY} ]]; then
    >&2 echo "ERROR: REGISTRY is not set"
    exit 1
fi

REGISTRY_PATH="${REGISTRY}/deckhouse/binaries"

VERSION="1.20.1"
for DISTRO in focal bionic xenial; do
  mkdir package
  pushd package
  wget "https://nginx.org/packages/ubuntu/pool/nginx/n/nginx/nginx_${VERSION}-1~${DISTRO}_amd64.deb"
  DEB_PACKAGE_FILE="$(ls nginx*)"
  popd

  cat <<EOF > package/install
#!/bin/bash
set -Eeo pipefail
dpkg -i -E ${DEB_PACKAGE_FILE}
apt-mark hold nginx
EOF
  chmod +x package/install

  cat <<EOF > package/uninstall
#!/bin/bash
set -Eeo pipefail
apt-mark unhold nginx
dpkg -r nginx
EOF
  chmod +x package/uninstall

  cat <<EOF > Dockerfile
FROM scratch
COPY ./package/* /
EOF

  docker build -t ${REGISTRY_PATH}/nginx:${VERSION}-${DISTRO} .
  docker push ${REGISTRY_PATH}/nginx:${VERSION}-${DISTRO}
  rm -rf package Dockerfile
done
