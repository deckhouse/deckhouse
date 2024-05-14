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

bb-package-install "jq:{{ .images.registrypackages.jq16 }}" "curl:{{ .images.registrypackages.d8Curl821 }}" "virt-what:{{ .images.registrypackages.virtWhat125 }}" "socat:{{ .images.registrypackages.socat1734 }}" "e2fsprogs:{{ .images.registrypackages.e2fsprogs1470 }}" "mount:{{ .images.registrypackages.utilLinux2401 }}" "xfsprogs:{{ .images.registrypackages.xfsprogs670 }}"
