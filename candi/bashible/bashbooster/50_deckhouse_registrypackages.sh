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

# shellcheck disable=SC2211,SC2153

# TODO remove this file in the release after 1.59 after migrate to use bb-package-* functions in the all external modules

bb-var BB_RP_INSTALLED_PACKAGES_STORE "/var/cache/registrypackages"

BB_RP_CURL_COMMON_ARGS=(
  --connect-timeout 10
  --max-time 300
  --retry 3
)

# Use d8-curl if installed, fallback to system package if not
bb-rp-curl() {
  if command -v d8-curl > /dev/null ; then
    d8-curl "${BB_RP_CURL_COMMON_ARGS[@]}" -4 --remove-on-error --parallel "$@"
  else
    curl "${BB_RP_CURL_COMMON_ARGS[@]}" "$@"
  fi
}
