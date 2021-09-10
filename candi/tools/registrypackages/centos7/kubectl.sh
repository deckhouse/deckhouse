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
YUMDATA="$(curl https://packages.cloud.google.com/yum/repos/kubernetes-el7-x86_64/repodata/primary.xml | grep "location href=" | awk -F "\"" '{print $2}')"

for VERSION in $(yq r /deckhouse/candi/version_map.yml -j | jq -r '.k8s | keys[] as $key | "\($key).\(.[$key] | .patch)"'); do
  mkdir package
  pushd package
  RPM_PACKAGE="https://packages.cloud.google.com/yum/repos/kubernetes-el7-x86_64/$(grep "kubectl-${VERSION}-0" <<< ${YUMDATA})"
  wget ${RPM_PACKAGE}
  RPM_PACKAGE_FILE="$(ls *kubectl*)"
  popd

  cat <<EOF > package/install
#!/bin/bash
set -Eeo pipefail
rpm -U ${RPM_PACKAGE_FILE}
yum versionlock add kubectl
EOF
  chmod +x package/install

  cat <<EOF > package/uninstall
#!/bin/bash
set -Eeo pipefail
yum versionlock delete kubectl
rpm -e kubectl
EOF
  chmod +x package/uninstall

  cat <<EOF > Dockerfile
FROM scratch
COPY ./package/* /
EOF

  docker build -t ${REGISTRY_PATH}/kubectl:${VERSION}-centos7 .
  docker push ${REGISTRY_PATH}/kubectl:${VERSION}-centos7
  rm -rf package Dockerfile
done
