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

SYSTEM_PACKAGES="curl wget inotify-tools bash-completion lvm2 parted apt-transport-https sudo nfs-common vim libseccomp2"
KUBERNETES_DEPENDENCIES="iptables iproute2 socat util-linux mount ebtables ethtool"

if bb-is-debian-version? 9 || bb-is-debian-version? 10 || bb-is-debian-version? 11; then
  SYSTEM_PACKAGES="${SYSTEM_PACKAGES} virt-what"
  KUBERNETES_DEPENDENCIES="${KUBERNETES_DEPENDENCIES} conntrack"
else
  bb-rp-install "virt-what:{{ .images.registrypackages.virtWhatDebian1151Deb9u1 }}" "conntrack:{{ .images.registrypackages.conntrackDebian1462 }}"
fi

bb-apt-install ${SYSTEM_PACKAGES} ${KUBERNETES_DEPENDENCIES}

bb-rp-install "jq:{{ .images.registrypackages.jq16 }}" "curl:{{ .images.registrypackages.d8Curl7800 }}"
