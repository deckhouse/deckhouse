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

modprobe br_netfilter
modprobe overlay

bb-sync-file /etc/modules-load.d/d8_br_netfilter.conf - <<< "br_netfilter"
bb-sync-file /etc/modules-load.d/d8_overlay.conf - <<< "overlay"

{{- if eq .cri "ContainerdV2" }}
if ! modprobe erofs; then
    bb-log-error "Error: failed to load erofs kernel module"
    exit 1
fi
bb-sync-file /etc/modules-load.d/d8_erofs.conf - <<< "erofs"
{{- end }}
