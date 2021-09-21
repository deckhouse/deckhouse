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
mkdir package
pushd package
wget "https://nginx.org/packages/centos/7/x86_64/RPMS/nginx-${VERSION}-1.el7.ngx.x86_64.rpm"
RPM_PACKAGE_FILE="$(ls nginx*)"
popd

cat <<EOF > package/install
#!/bin/bash
set -Eeo pipefail
rpm -U ${RPM_PACKAGE_FILE}
yum versionlock add nginx
EOF
chmod +x package/install

cat <<EOF > package/uninstall
#!/bin/bash
set -Eeo pipefail
yum versionlock delete nginx
rpm -e ${RPM_PACKAGE_FILE%.rpm}
EOF
chmod +x package/uninstall

cat <<EOF > Dockerfile
FROM scratch
COPY ./package/* /
EOF

docker build -t ${REGISTRY_PATH}/nginx:${VERSION}-centos7 .
docker push ${REGISTRY_PATH}/nginx:${VERSION}-centos7
rm -rf package Dockerfile
