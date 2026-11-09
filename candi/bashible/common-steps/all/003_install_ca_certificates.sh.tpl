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
# bashible: parallel-group=light-prep

bb-package-wait d8-ca-updater

mkdir -p /opt/deckhouse/share/ca-certificates/

{{- range $_, $host_values := .registry.hosts }}
  {{- range $mirror := $host_values.mirrors }}
    {{- if $mirror.ca }}

bb-log-info "Sync registry CA for {{ $mirror.host }}"
bb-sync-file "/opt/deckhouse/share/ca-certificates/registry-{{ $mirror.host | lower }}-ca.crt" - << "EOF"
{{ $mirror.ca }}
EOF
    {{- end }}
  {{- end }}
{{- end }}

{{- /*
  The authorities of the RegistryUpstream objects step 032 renders inline: only containerd v1
  reads them from here, while the registry module configures no node (disabled, or in
  Unmanaged mode), since step 032 then sets no config_path. Every other runtime, and
  containerd v1 while the module configures it, reads registry.d, where nodelet writes them.
  Their hosts never repeat one above (bashible-apiserver, pkg/template/registry/upstreams.go).
*/}}
{{- if and (eq .cri "Containerd") (not .registry.registryModuleEnable) }}
  {{- range $_, $host_values := (.registry.inlineUpstreams | default dict) }}
    {{- range $mirror := $host_values.mirrors }}
      {{- if $mirror.ca }}

bb-log-info "Sync registry CA for {{ $mirror.host }}"
bb-sync-file "/opt/deckhouse/share/ca-certificates/registry-{{ $mirror.host | lower }}-ca.crt" - << "EOF"
{{ $mirror.ca }}
EOF
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}
