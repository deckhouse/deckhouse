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

for VERSION in $(yq r /deckhouse/candi/version_map.yml -j | jq -r '.k8s | .[].bashible.centos."7"' | grep desiredVersion | grep docker | awk '{print $2}' | tr -d '"' | sort | uniq); do
  PACKAGE="$(sed "s/docker-ce-/docker-ce:/" <<< "${VERSION}")"
  VERSION_CLI="$(sed "s/docker-ce-/docker-ce-cli-/" <<< "${VERSION}")"
  mkdir package
  pushd package
  # Centos
  # get url with yumdownloader --urls
  RPM_PACKAGE="https://download.docker.com/linux/centos/7/x86_64/stable/Packages/${VERSION}.rpm"
  RPM_PACKAGE_CLI="https://download.docker.com/linux/centos/7/x86_64/stable/Packages/${VERSION_CLI}.rpm"
  wget ${RPM_PACKAGE}
  RPM_PACKAGE_FILE="$(ls docker-ce*)"
  wget ${RPM_PACKAGE_CLI}
  RPM_PACKAGE_CLI_FILE="$(ls docker-ce-cli*)"
  popd
  cat <<EOF > package/install
#!/bin/bash
set -Eeo pipefail
rpm -U ${RPM_PACKAGE_FILE} ${RPM_PACKAGE_CLI_FILE}
yum versionlock add docker-ce docker-ce-cli
EOF
  chmod +x package/install

  cat <<EOF > package/uninstall
#!/bin/bash
set -Eeo pipefail
yum versionlock delete docker-ce docker-ce-cli
rpm -e ${RPM_PACKAGE_CLI_FILE%.rpm} ${RPM_PACKAGE_FILE%.rpm}
EOF
  chmod +x package/uninstall

  cat <<EOF > Dockerfile
FROM scratch
COPY ./package/* /
EOF

  docker build -t ${REGISTRY_PATH}/${PACKAGE} .
  docker push ${REGISTRY_PATH}/${PACKAGE}
  rm -rf package Dockerfile
done
