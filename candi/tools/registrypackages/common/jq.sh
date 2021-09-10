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

VERSION="1.6"
mkdir package
curl -sL https://github.com/stedolan/jq/releases/download/jq-${VERSION}/jq-linux64 --output package/jq
chmod +x package/jq

cat <<EOF > package/install
#!/bin/bash
set -Eeo pipefail
cp jq /usr/local/bin
EOF
chmod +x package/install

cat <<EOF > package/uninstall
#!/bin/bash
set -Eeo pipefail
rm -f /usr/local/bin/jq
EOF
chmod +x package/uninstall

cat <<EOF > Dockerfile
FROM scratch
COPY ./package/* /
EOF

docker build -t ${REGISTRY_PATH}/jq:${VERSION} .
docker push ${REGISTRY_PATH}/jq:${VERSION}
rm -rf package Dockerfile
