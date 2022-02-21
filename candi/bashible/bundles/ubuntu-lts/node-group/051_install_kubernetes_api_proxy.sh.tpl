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

if bb-is-ubuntu-version? 16.04 ; then
  bb-rp-install "nginx:{{ .images.registrypackages.nginxUbuntu1201Xenial }}"
fi
if bb-is-ubuntu-version? 18.04 ; then
  bb-rp-install "nginx:{{ .images.registrypackages.nginxUbuntu1202Bionic }}"
fi
if bb-is-ubuntu-version? 20.04 ; then
  bb-rp-install "nginx:{{ .images.registrypackages.nginxUbuntu1202Focal }}"
fi
