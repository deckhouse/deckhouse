{{- /*
# Copyright 2024 Flant JSC
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
*/}}
#!/bin/bash
set -Eeo pipefail

# candi/bashible/bootstrap/01-bootstrap-prerequisites.sh.tpl

{{- $packagesProxy := .packagesProxy | default (dict) }}
{{- $clusterMasterEndpoints := .clusterMasterEndpoints | default (list) }}
{{- $clusterMasterKubeAPIEndpoints := list }}
{{- $clusterMasterRPPAddresses := list }}
{{- $clusterMasterBootstrapRPPAddresses := list }}
{{- range $endpoint := $clusterMasterEndpoints }}
  {{- $address := get $endpoint "address" }}
  {{- if hasKey $endpoint "kubeApiPort" }}
    {{- $clusterMasterKubeAPIEndpoints = append $clusterMasterKubeAPIEndpoints (printf "%s:%v" $address (get $endpoint "kubeApiPort")) }}
  {{- end }}
  {{- $clusterMasterRPPAddresses = append $clusterMasterRPPAddresses (printf "%s:%v" $address (get $endpoint "rppServerPort")) }}
  {{- $clusterMasterBootstrapRPPAddresses = append $clusterMasterBootstrapRPPAddresses (printf "%s:%v" $address (get $endpoint "rppBootstrapServerPort")) }}
{{- end }}
{{- $candi := "candi/bashible/lib.sh.tpl" -}}
{{- $deckhouse := "/deckhouse/candi/bashible/lib.sh.tpl" -}}
{{- $lib := .Files.Get $deckhouse | default (.Files.Get $candi) -}}
{{- $ctx := . -}}
{{- tpl (printf `
%s

{{ template "bb-minget" $ }}

` $lib) $ctx }}

export PACKAGES_PROXY_BOOTSTRAP_CLUSTER_UUID="{{ .clusterUUID | default "" }}"
export PACKAGES_PROXY_BOOTSTRAP_ADDRESSES="{{ $clusterMasterBootstrapRPPAddresses | join " " }}"

{{ if gt (len $clusterMasterKubeAPIEndpoints) 0 }}
# autodiscover token and rpp endpoint
export PACKAGES_PROXY_KUBE_APISERVER_ENDPOINTS="{{ $clusterMasterKubeAPIEndpoints | join "," }}"
{{ else }}
# static set of rpp endpoint and token
export PACKAGES_PROXY_TOKEN="{{ get $packagesProxy "token" | default "passthrough" }}"
export PACKAGES_PROXY_ADDRESSES="{{ $clusterMasterRPPAddresses | join "," }}"
{{ end }}

# packages

bb-minget-install
bb-rpp-get-install

{{ with .images.registrypackages }}
/opt/deckhouse/bin/rpp-get install "jq:{{ .jq171 }}" "curl:{{ .d8Curl891 }}" "tailLog:{{ .tailLog }}"
{{- end }}

# network

{{- if and (ne .nodeGroup.nodeType "Static") (.provider )}}
  {{- if $bootstrap_script_network := $.Files.Get (printf "deckhouse/candi/cloud-providers/%s/bashible/bootstrap-networks.sh.tpl" .provider) | default ($.Files.Get (printf "candi/cloud-providers/%s/bashible/bootstrap-networks.sh.tpl" .provider) ) }}
    {{- tpl ($bootstrap_script_network) $ | nindent 0 }}
  {{- end }}
{{- end }}
