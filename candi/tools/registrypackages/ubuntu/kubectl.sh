# Copyright 2021 Flant CJSC
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

for VERSION in $(yq r /deckhouse/candi/version_map.yml -j | jq -r '.k8s | keys[] as $key | "\($key).\(.[$key] | .patch)"'); do
  mkdir package
  pushd package
  DEB_PACKAGE="https://packages.cloud.google.com/apt/$(curl https://packages.cloud.google.com/apt/dists/kubernetes-xenial/main/binary-amd64/Packages | grep kubectl_${VERSION}-00  | awk '{print $2}')"
  wget ${DEB_PACKAGE}
  DEB_PACKAGE_FILE="$(ls kubectl*)"
  popd

  cat <<EOF > package/install
#!/bin/bash
set -Eeo pipefail
dpkg -i -E ${DEB_PACKAGE_FILE}
apt-mark hold kubectl
EOF
  chmod +x package/install

  cat <<EOF > package/uninstall
#!/bin/bash
set -Eeo pipefail
apt-mark unhold kubectl
dpkg -r kubectl
EOF
  chmod +x package/uninstall

  cat <<EOF > Dockerfile
FROM scratch
COPY ./package/* /
EOF

  docker build -t ${REGISTRY_PATH}/kubectl:${VERSION}-ubuntu .
  docker push ${REGISTRY_PATH}/kubectl:${VERSION}-ubuntu
  rm -rf package Dockerfile
done
