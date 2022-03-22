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

# just a acomment
set -e

deckhouseVer="dev"
shellOpVer=$(go list -m all | grep shell-operator | cut -d' ' -f 2-)
addonOpVer=$(go list -m all | grep addon-operator | cut -d' ' -f 2-)

jqRoot=$1
if [ "x${jqRoot}" = "x" ]; then
  >&2 echo "A path to libjq static libraries should be specified!"
  exit 1
fi

# Can be removed when Go 1.16 will be in use.
export GO111MODULE=on

CGO_ENABLED=1 \
    CGO_CFLAGS="-I${jqRoot}/include" \
    CGO_LDFLAGS="-L${jqRoot}/lib" \
    GOOS=linux \
    go build \
     -ldflags="-s -w -X 'main.DeckhouseVersion=$deckhouseVer' -X 'main.AddonOperatorVersion=$addonOpVer' -X 'main.ShellOperatorVersion=$shellOpVer'" \
     -o ./deckhouse-controller \
     ./cmd/deckhouse-controller
