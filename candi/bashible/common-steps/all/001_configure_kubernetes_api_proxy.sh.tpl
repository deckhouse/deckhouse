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
# Read previously discovered IP

bb-sync-file /etc/kubernetes/kubernetes-api-proxy/upstreams.json - < EOF
{{- $list := list }}
{{- if eq .runType "Normal" }}
  {{- range $key, $value := .normal.apiserverEndpoints }}
    {{- $list = append $list $value }}
  {{- end }}
{{- else if eq .runType "ClusterBootstrap" }}
    {{- $list = append $list "$(bb-d8-node-ip):6443" }}
{{- end }}
{{ toJson $list }}
EOF

{{ /* TODO: Does we can run it only on Normal execution? */ }}
{{ if eq .runType "Normal" }}
if [! -f /etc/kubernetes/kubernetes-api-proxy/ca.crt ]; then
  bb-sync-file /etc/kubernetes/kubernetes-api-proxy/ca.crt - < EOF
  {{- /* TODO: We need to put ca to BashibleContext, and here we can push it to file */ -}}
  EOF
fi

if [! -f /etc/kubernetes/kubernetes-api-proxy/cl.crt ]; then
  bb-sync-file /etc/kubernetes/kubernetes-api-proxy/cl.crt - < EOF
  {{- /* TODO: We need to put crt to BashibleContext, and here we can push it to file */ -}}
  EOF
fi

if [! -f /etc/kubernetes/kubernetes-api-proxy/cl.key ]; then
  bb-sync-file /etc/kubernetes/kubernetes-api-proxy/cl.key - < EOF
  {{- /* TODO: We need to put key to BashibleContext, and here we can push it to file */ -}}
  EOF
fi

{{- end }}

chown -R 0:64535 /etc/kubernetes/kubernetes-api-proxy
chmod g+s /etc/kubernetes/kubernetes-api-proxy
chmod 750 /etc/kubernetes/kubernetes-api-proxy
chmod 640 /etc/kubernetes/kubernetes-api-proxy/*
