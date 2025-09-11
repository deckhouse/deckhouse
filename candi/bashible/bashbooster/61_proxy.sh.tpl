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

bb-set-proxy() {
{{- if .proxy }}
  {{- if .proxy.httpProxy }}
  export HTTP_PROXY={{ .proxy.httpProxy | quote }}
  export http_proxy=${HTTP_PROXY}
  {{- end }}
  {{- if .proxy.httpsProxy }}
  export HTTPS_PROXY={{ .proxy.httpsProxy | quote }}
  export https_proxy=${HTTPS_PROXY}
  {{- end }}
  {{- $noProxy := list "127.0.0.1" "169.254.169.254" .Values.global.clusterConfiguration.clusterDomain .Values.global.clusterConfiguration.podSubnetCIDR .Values.global.clusterConfiguration.serviceSubnetCIDR }}
  {{- if .proxy.noProxy }}
    {{- $noProxy = concat $noProxy .proxy.noProxy }}
  {{- end }}
  export NO_PROXY={{ $noProxy | join "," | quote }}
  export no_proxy=${NO_PROXY}
{{- else }}
  unset HTTP_PROXY http_proxy HTTPS_PROXY https_proxy NO_PROXY no_proxy
{{- end }}
}

bb-unset-proxy() {
unset HTTP_PROXY http_proxy HTTPS_PROXY https_proxy NO_PROXY no_proxy
}
