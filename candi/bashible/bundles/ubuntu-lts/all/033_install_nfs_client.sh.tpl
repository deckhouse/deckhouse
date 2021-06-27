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

if bb-is-ubuntu-version? 20.04 ; then
  bb-apt-install "nfs-common=1:1.3.4-2.5ubuntu3.*"
elif bb-is-ubuntu-version? 18.04 ; then
  bb-apt-install "nfs-common=1:1.3.4-2.1ubuntu5.*"
elif bb-is-ubuntu-version? 16.04 ; then
  bb-apt-install "nfs-common=1:1.2.8-9ubuntu12.*"
else
  bb-log-error "Unsupported Ubuntu version"
  exit 1
fi
