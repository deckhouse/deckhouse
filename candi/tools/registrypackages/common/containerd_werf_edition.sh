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

VERSION="v1.4.6+werf-fix.2"
IMAGE_VERSION="$(sed "s/+/-/" <<< "${VERSION}")"
mkdir package
curl -L https://github.com/flant/containerd/releases/download/${VERSION}/containerd --output package/containerd
chmod +x package/containerd

cat <<EOF > package/install
#!/bin/bash
set -Eeo pipefail
cp containerd /usr/local/bin
EOF
chmod +x package/install

cat <<EOF > package/uninstall
#!/bin/bash
set -Eeo pipefail
rm -f /usr/local/bin/containerd
EOF
chmod +x package/uninstall

cat <<EOF > Dockerfile
FROM scratch
COPY ./package/* /
EOF

docker build -t ${REGISTRY_PATH}/containerd-werf-edition:${IMAGE_VERSION} .
docker push ${REGISTRY_PATH}/containerd-werf-edition:${IMAGE_VERSION}
rm -rf package Dockerfile
