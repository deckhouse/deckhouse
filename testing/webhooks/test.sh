#!/bin/bash

# Copyright 2024 Flant JSC
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
set -Eeo pipefail

apt-get update && apt-get install -y python3 python3-module-pip-run

pip3 install -r /requirements.txt

mkdir /tests

find /src -wholename '*/webhooks/*.py' -exec sh -c 'module="$(echo "$1" | cut -d / -f 3)"; mkdir -p "/tests/${module}"; cp "$1" "/tests/${module}"' sh {}  \;

cd /tests

find . -wholename '*_test.py' -print0 | xargs -n 1 -t --null python3
