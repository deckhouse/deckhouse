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

discovered_node_ip="$(</var/lib/bashible/discovered-node-ip)"

{{- range $host := .registry.hosts }}
  {{- if not (has $host.host $exist_registry_host_list) }}
    {{- $exist_registry_host_list = append $exist_registry_host_list $host.host }}
  {{- end }}

mkdir -p "/etc/containerd/registry.d/{{ $host.host }}"
  {{- $ca_files_path := list }}
  {{- range $index, $CA := $host.ca }}
  {{- $ca_file_path := printf "/etc/containerd/registry.d/%s/ca_%d.crt" $host.host $index }}
  {{- $ca_files_path = append $ca_files_path $ca_file_path }}
bb-sync-file {{ $ca_file_path | quote }} - << EOF
{{ $CA }}
EOF
  {{- end }}

bb-sync-file "/etc/containerd/registry.d/{{ $host.host }}/hosts.toml" - << EOF
[host]
{{- range $mirror := $host.mirrors }}
  {{- $mirrorHostWithScheme := (printf "%s://%s" $mirror.scheme $mirror.host) }}

  [host.{{ $mirrorHostWithScheme | quote }}]
  capabilities = ["pull", "resolve"]
  {{- if gt (len $ca_files_path) 0 }}
  ca = {{- printf "[%q]" (join "\", \"" $ca_files_path) }}
  {{- end }}

    {{- if or $mirror.auth $mirror.username $mirror.password }}
    [host.{{ $mirrorHostWithScheme | quote }}.auth]
      {{- if $mirror.auth }}
    auth = {{ $mirror.auth | quote }}
      {{- else }}
    username = {{ $mirror.username | quote }}
    password = {{ $mirror.password | quote }}
      {{- end }}
    {{- end }}
{{- end }}
EOF

{{- end }}

{{- if eq .runType "Normal" }}
  {{- range $host, $CA := .normal.moduleSourcesCA }}
    {{- if not (has $host $exist_registry_host_list) }}
      {{- if $CA }}
      {{- $exist_registry_host_list = append $exist_registry_host_list $host }}

# Sync module sources host.toml and ca.crt
mkdir -p "/etc/containerd/registry.d/{{ $host }}"
bb-sync-file "/etc/containerd/registry.d/{{ $host }}/ca.crt" - << EOF
{{ $CA }}
EOF

bb-sync-file "/etc/containerd/registry.d/{{ $host }}/hosts.toml" - << EOF
# Server specifies the default server for this registry host namespace.
# When host(s) are specified, the hosts are tried first in the order listed.
# https://github.com/containerd/containerd/blob/v1.7.24/docs/hosts.md#hoststoml-content-description---detail

server = {{ $host | quote }}
ca = ["/etc/containerd/registry.d/{{ $host }}/ca.crt"]

[host]
EOF
      {{- end }}
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
