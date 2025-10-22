#!/bin/sh

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

set -e

deckhouseVer=${D8_VERSION:-"dev"}
defaultKubernetesVer=${DEFAULT_KUBERNETES_VERSION}
defaultReleaseChannel=${DEFAULT_RELEASE_CHANNEL}
shellOpVer=$(go list -m all | grep shell-operator | cut -d' ' -f 2-)
addonOpVer=$(go list -m all | grep addon-operator | cut -d' ' -f 2-)

# Validate required variables
if [ -z "${defaultKubernetesVer}" ]; then
  echo "DEFAULT_KUBERNETES_VERSION is not set"
  exit 1
fi

# Build ldflags
LDFLAGS="-s -w"
LDFLAGS="${LDFLAGS} -X 'main.DeckhouseVersion=${deckhouseVer}'"
LDFLAGS="${LDFLAGS} -X 'main.AddonOperatorVersion=${addonOpVer}'"
LDFLAGS="${LDFLAGS} -X 'main.ShellOperatorVersion=${shellOpVer}'"
LDFLAGS="${LDFLAGS} -X 'main.DefaultReleaseChannel=${defaultReleaseChannel}'"
LDFLAGS="${LDFLAGS} -X 'github.com/deckhouse/deckhouse/modules/040-control-plane-manager/hooks.DefaultKubernetesVersion=${defaultKubernetesVer}'"

# Build the binary
CGO_ENABLED=0 \
GOOS=linux \
GOARCH=amd64 \
    go build \
        -ldflags="${LDFLAGS}" \
        -o ./deckhouse-controller \
        ./cmd/deckhouse-controller
