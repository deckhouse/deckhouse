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

{{- if eq .cri "Containerd" }}
{{- $existRegistryHostList := list .registry.address }}

# Sync current registry host.toml and ca.crt
mkdir -p "/etc/containerd/registry.d/{{ .registry.address }}"

{{- if .registry.ca }}
bb-sync-file "/etc/containerd/registry.d/{{ .registry.address }}/ca.crt" - << EOF
{{ .registry.ca }}
EOF
{{- end }}

bb-sync-file "/etc/containerd/registry.d/{{ .registry.address }}/hosts.toml" - << EOF
{{- $host := (printf "%s://%s" .registry.scheme .registry.address) }}
{{- if ne .registry.registryMode "Direct" }}
  {{- $host = "https://127.0.0.1:5001" }}
{{- end }}

[host]
  [host.{{ $host | quote }}]
  capabilities = ["pull", "resolve"]
  {{- if .registry.ca }}
  ca = ["/etc/containerd/registry.d/{{ .registry.address }}/ca.crt"]
  {{- end }}

    {{- if .registry.auth }}
    [host.{{ $host | quote }}.auth]
    auth = {{ .registry.auth | quote }}
    {{- end }}
EOF


{{- if .systemRegistry.registryAddress }}
  {{- if not (has .systemRegistry.registryAddress $existRegistryHostList) }}
  {{- $existRegistryHostList = append $existRegistryHostList .systemRegistry.registryAddress }}

# Sync embedded registry host.toml and ca.crt
mkdir -p "/etc/containerd/registry.d/{{ .systemRegistry.registryAddress }}"

{{- if .systemRegistry.registryCA }}
bb-sync-file "/etc/containerd/registry.d/{{ .systemRegistry.registryAddress }}/ca.crt" - << EOF
{{ .systemRegistry.registryCA }}
EOF
{{- end }}

bb-sync-file "/etc/containerd/registry.d/{{ .systemRegistry.registryAddress }}/hosts.toml" - << EOF
{{- $host := "https://127.0.0.1:5001" }}

[host]
  [host.{{ $host | quote }}]
  capabilities = ["pull", "resolve"]
  {{- if .systemRegistry.registryCA }}
  ca = ["/etc/containerd/registry.d/{{ .systemRegistry.registryAddress }}/ca.crt"]
  {{- end }}

    {{- if .systemRegistry.auth }}
    [host.{{ $host | quote }}.auth]
    auth = {{ .systemRegistry.auth | quote }}
    {{- end }}
EOF
  {{- end }}
{{- end }}


{{- if eq .runType "Normal" }}
  {{- range $registryHost, $registryCA := .normal.moduleSourcesCA }}
    {{- if not (has $registryHost $existRegistryHostList) }}
      {{- if $registryCA }}
      {{- $existRegistryHostList = append $existRegistryHostList $registryHost }}

# Sync module sources host.toml and ca.crt
mkdir -p "/etc/containerd/registry.d/{{ $registryHost }}"
bb-sync-file "/etc/containerd/registry.d/{{ $registryHost }}/ca.crt" - << EOF
{{ $registryCA }}
EOF

bb-sync-file "/etc/containerd/registry.d/{{ $registryHost }}/hosts.toml" - << EOF
server = {{ $registryHost | quote }}
ca = ["/etc/containerd/registry.d/{{ $registryHost }}/ca.crt"]

[host]
EOF
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}


# Manage old host directories and update state
hosts_state_file="/etc/containerd/registry.d/deckhouse_hosts_state.json"
new_hosts='{{ $existRegistryHostList | uniq | toJson }}'
old_hosts="[]"
if [[ -f "$hosts_state_file" ]]; then
  old_hosts=$(< "$hosts_state_file")
fi

# Remove old hosts
echo "$old_hosts" | /opt/deckhouse/bin/jq -r --argjson new_hosts "$new_hosts" '
  .[] | select(. as $host | $new_hosts | index($host) | not)' | while IFS= read -r old_host; do
  host_dir="/etc/containerd/registry.d/$old_host"
  if [[ -d "$host_dir" ]]; then
    rm -rf "$host_dir"
  fi
done

# Updated state
echo "$new_hosts" > "$hosts_state_file"

{{- end }}
