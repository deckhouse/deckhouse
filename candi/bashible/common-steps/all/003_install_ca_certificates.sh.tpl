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

bb-package-install "d8-ca-updater:{{ .images.registrypackages.d8CaUpdater200225 }}"

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
