#!/bin/bash

# Copyright 2025 Flant JSC
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

echo "Update ratify crds"
current_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
script_path=$(dirname "${BASH_SOURCE[0]}")
version=$(cat $script_path/../../images/ratify/werf.inc.yaml | grep "ratifyVersion :=" | sed -n 's/.*"\(.*\)".*/\1/p')
echo Ratify version: $version
git clone --depth 1 --branch  $version  https://github.com/notaryproject/ratify.git /tmp/ratify
rm $script_path/*.yaml
cp /tmp/ratify/config/crd/bases/*.yaml "${script_path}"
rm -rf /tmp/ratify
