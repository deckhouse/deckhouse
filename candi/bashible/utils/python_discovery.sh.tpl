{{- /*
# Copyright 2023 Flant JSC
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
*/}}

python_binary=""

for pybin in python3 python2 python; do
  if command -v "$pybin" >/dev/null 2>&1; then
    python_binary="$pybin"
    break
  fi
done

if [ -z "$python_binary" ]; then
  echo "Python binary not found"
  exit 1
fi
