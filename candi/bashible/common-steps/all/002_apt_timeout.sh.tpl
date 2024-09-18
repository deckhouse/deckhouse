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

function set_apt_timeout() {
  if [[ -f /etc/apt/apt.conf.d/99timeout ]]; then
    return 0
  fi

  echo 'Acquire::http::Timeout "120";' > /etc/apt/apt.conf.d/99timeout
}

case $(bb-is-bundle) in
  debian|ubuntu-lts|astra|altlinux) set_apt_timeout ;;
esac


