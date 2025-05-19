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
{{- $exist_registry_host_list := list }}

# PR: https://github.com/deckhouse/deckhouse/pull/11939
#
# Enabled by settings in /etc/containerd/config.toml:
# ```
# [plugins."io.containerd.grpc.v1.cri".registry]
#   config_path = "/etc/containerd/registry.d"
# ```
#
# Example structure for /etc/containerd/registry.d:
# ```
# .
# ├── deckhouse_hosts_state.json
# ├── embedded-registry.d8-system.svc:5001
# │   ├── ca.crt
# │   └── hosts.toml
# └── registry.deckhouse.ru
#     ├── ca.crt
#     └── hosts.toml
# ```
#
# Operational principles:
# - Create directories for hosts.toml configurations
# - Remove old directories (based on information from deckhouse_hosts_state.json)
# - Save the new state to deckhouse_hosts_state.json

discovered_node_ip="$(bb-d8-node-ip)"

{{- range $hostName, $hostValues := .registry.hosts }}
  {{- if not (has $hostName $exist_registry_host_list) }}
    {{- $exist_registry_host_list = append $exist_registry_host_list $hostName }}
  {{- end }}

mkdir -p "/etc/containerd/registry.d/{{ $hostName }}"
  {{- $ca_files_path := list }}
  {{- range $index, $CA := $hostValues.ca }}
  {{- $ca_file_path := printf "/etc/containerd/registry.d/%s/ca_%d.crt" $hostName $index }}
  {{- $ca_files_path = append $ca_files_path $ca_file_path }}
bb-sync-file {{ $ca_file_path | quote }} - << EOF
{{ $CA }}
EOF
  {{- end }}

bb-sync-file "/etc/containerd/registry.d/{{ $hostName }}/hosts.toml" - << EOF
[host]
{{- range $mirror := $hostValues.mirrors }}
  {{- $mirrorHostWithScheme := (printf "%s://%s" $mirror.scheme $mirror.host) }}

  [host.{{ $mirrorHostWithScheme | quote }}]
  capabilities = ["pull", "resolve"]
  {{- if and (eq $mirror.scheme "https") (gt (len $ca_files_path) 0) }}
  ca = [{{- range $i, $path := $ca_files_path }}{{ if $i }}, {{ end }}{{ $path | quote }}{{- end }}]
  {{- end }}

    {{- with $mirror.auth }}
      {{- if or .auth .username .password }}
    [host.{{ $mirrorHostWithScheme | quote }}.auth]
        {{- if .auth }}
    auth = {{ .auth | quote }}
        {{- else }}
    username = {{ .username | quote }}
    password = {{ .password | quote }}
        {{- end }}
      {{- end }}
    {{- end }}

    {{- range $mirror.rewrites }}
    [[host.{{ $mirrorHostWithScheme | quote }}.rewrite]]
    regex = {{ .from | quote }}
    replace = {{ .to | quote }}
    {{- end }}
{{- end }}
EOF

{{- end }}

{{- if eq .runType "Normal" }}
  {{- range $hostName, $CA := .normal.moduleSourcesCA }}
    {{- if and (not (has $hostName $exist_registry_host_list)) $CA }}
      {{- $exist_registry_host_list = append $exist_registry_host_list $hostName }}

# Sync module sources host.toml and ca.crt
mkdir -p "/etc/containerd/registry.d/{{ $hostName }}"
bb-sync-file "/etc/containerd/registry.d/{{ $hostName }}/ca.crt" - << EOF
{{ $CA }}
EOF

bb-sync-file "/etc/containerd/registry.d/{{ $hostName }}/hosts.toml" - << EOF
server = {{ $hostName | quote }}
ca = ["/etc/containerd/registry.d/{{ $hostName }}/ca.crt"]

[host]
EOF
    {{- end }}
  {{- end }}
{{- end }}


# Manage old host directories and update state
hosts_state_file="/etc/containerd/registry.d/deckhouse_hosts_state.json"
new_hosts='{{ $exist_registry_host_list | uniq | toJson }}'
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
