# Copyright 2025 Flant JSC
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

{{- with (.registry).init }}

# Create init registry config file
INIT_CONFIG_PATH="$(bb-tmp-file)"
bb-sync-file $INIT_CONFIG_PATH - << "EOF"
{{ . | toYaml }}
EOF

# Create d8-system namespace
bb-kubectl --kubeconfig=/etc/kubernetes/admin.conf get ns d8-system || bb-kubectl --kubeconfig=/etc/kubernetes/admin.conf create ns d8-system

# Upload init registry secret
bb-kubectl --kubeconfig=/etc/kubernetes/admin.conf -n d8-system delete secret registry-init || true
bb-kubectl --kubeconfig=/etc/kubernetes/admin.conf -n d8-system create secret generic registry-init \
  --from-file=config=$INIT_CONFIG_PATH

{{- end }}
