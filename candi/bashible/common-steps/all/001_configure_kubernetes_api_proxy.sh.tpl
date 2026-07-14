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

mkdir -p /etc/kubernetes/kubernetes-api-proxy

{{- if eq .runType "Normal" }}
upstreams_json='{{ .clusterMasterKubeAPIEndpoints | toJson }}'
if ! jq -e 'arrays | length > 0' <<< "${upstreams_json}" > /dev/null 2>&1; then
  bb-log-error "clusterMasterKubeAPIEndpoints is empty or invalid, skipping upstreams.json update to prevent kube-api-proxy outage"
  exit 1
fi
bb-sync-file /etc/kubernetes/kubernetes-api-proxy/upstreams.json - << EOF
${upstreams_json}
EOF
{{- else if eq .runType "ClusterBootstrap" }}
node_endpoint="$(bb-d8-node-endpoint 6443)"
if [[ -z "$node_endpoint" ]]; then
  bb-log-error "bb-d8-node-endpoint returned empty value, cannot configure kubernetes-api-proxy upstreams.json"
  exit 1
fi
bb-sync-file /etc/kubernetes/kubernetes-api-proxy/upstreams.json - << EOF
[
  "${node_endpoint}"
]
EOF
{{- end }}

{{ if eq .runType "Normal" }}
bb-sync-file /etc/kubernetes/kubernetes-api-proxy/ca.crt - << EOF
{{ .normal.kubernetesCA }}
EOF

bb-sync-file /etc/kubernetes/kubernetes-api-proxy/cl.crt - << EOF
{{ .normal.apiserverProxyCerts.crt }}
EOF

bb-sync-file /etc/kubernetes/kubernetes-api-proxy/cl.key - << EOF
{{ .normal.apiserverProxyCerts.key }}
EOF
{{- end }}

chown -R 0:64535 /etc/kubernetes/kubernetes-api-proxy
chmod g+s /etc/kubernetes/kubernetes-api-proxy
chmod 750 /etc/kubernetes/kubernetes-api-proxy
chmod 640 /etc/kubernetes/kubernetes-api-proxy/*
