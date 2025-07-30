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

mkdir -p /opt/deckhouse/share/ca-certificates/

{{- if eq .runType "Normal" }}
	{{- range $registryAddr,$ca := .normal.moduleSourcesCA }}
		{{- if $ca }}

bb-log-info "Sync moduleSource CA for {{ $registryAddr }}"
bb-sync-file "/opt/deckhouse/share/ca-certificates/{{ $registryAddr | lower }}-ca.crt" - << "EOF"
{{ $ca }}
EOF
		{{- end }}
	{{- end }}
{{- end }}
