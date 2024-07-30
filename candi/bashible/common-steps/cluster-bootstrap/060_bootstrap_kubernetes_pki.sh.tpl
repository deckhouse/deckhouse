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

{{- if eq .runType "ClusterBootstrap" }}
# Read previously discovered IP
export MY_IP="$(</var/lib/bashible/discovered-node-ip)"

function subst_config() {
    tmpfile=$(mktemp /opt/deckhouse/tmp/kubeadm-config.XXXXXX)
    envsubst < "$1" > "$tmpfile"
    mv "$tmpfile" "$1"
}

subst_config /var/lib/bashible/kubeadm/config.yaml
for file in $(find /var/lib/bashible/kubeadm/patches/*.yaml); do
  subst_config "$file"
done
{{- end }}

kubeadm init phase certs ca --config /var/lib/bashible/kubeadm/config.yaml
