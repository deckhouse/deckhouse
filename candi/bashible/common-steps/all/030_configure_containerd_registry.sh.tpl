# Copyright 2025 Flant JSC
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

{{- if or ( eq .cri "Containerd") ( eq .cri "ContainerdV2") }}
{{- $exist_registry_host_list := list }}

# PR: https://github.com/deckhouse/deckhouse/pull/11939
#
# Enabled by settings in /etc/containerd/config.toml:
# Containerd v1:
# ```
# [plugins."io.containerd.grpc.v1.cri".registry]
#   config_path = "/etc/containerd/registry.d"
# ```
# Containerd v2:
# ```
# [plugins.'io.containerd.cri.v1.images'.registry]
#   config_path = "/etc/containerd/registry.d"
# ```
#
# Example structure for /etc/containerd/registry.d:
# ```
# .
# ├── deckhouse_hosts_state.json
# ├── registry.d8-system.svc:5001
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

{{- range $host_name, $host_values := .registry.hosts }}
  {{- if not (has $host_name $exist_registry_host_list) }}
    {{- $exist_registry_host_list = append $exist_registry_host_list $host_name }}
  {{- end }}

mkdir -p "/etc/containerd/registry.d/{{ $host_name }}"

# Create CA cert files for mirrors
{{- range $mirror := $host_values.mirrors }}
  {{- $mirror_ca_file_path := printf "/etc/containerd/registry.d/%s/%s-%s-ca.crt" $host_name $mirror.scheme $mirror.host }}
  {{- if $mirror.ca }}
bb-sync-file {{ $mirror_ca_file_path | quote }} - << EOF
{{ $mirror.ca }}
EOF
  {{- end }}
{{- end }}

# Create hosts.toml files for registries
bb-sync-file "/etc/containerd/registry.d/{{ $host_name }}/hosts.toml" - << EOF
[host]
{{- range $mirror := $host_values.mirrors }}
  {{- $mirror_host_with_scheme := (printf "%s://%s" $mirror.scheme $mirror.host) }}
  {{- $mirror_ca_file_path := printf "/etc/containerd/registry.d/%s/%s-%s-ca.crt" $host_name $mirror.scheme $mirror.host }}

  [host.{{ $mirror_host_with_scheme | quote }}]
    capabilities = ["pull", "resolve"]
    {{- if eq $mirror.scheme "http" }}
    skip_verify = true
    {{- end }}
    {{- if and (eq $mirror.scheme "https") $mirror.ca }}
    ca = [{{ $mirror_ca_file_path | quote }}]
    {{- end }}

    {{- with $mirror.auth }}
      {{- if or .auth .username }}
    [host.{{ $mirror_host_with_scheme | quote }}.auth]
        {{- if .auth }}
      auth = {{ .auth | quote }}
        {{- else }}
      username = {{ .username | quote }}
      password = {{ .password | default "" | quote }}
        {{- end }}
      {{- end }}
    {{- end }}

    {{- range $mirror.rewrites }}
    [[host.{{ $mirror_host_with_scheme | quote }}.rewrite]]
      regex = {{ .from | quote }}
      replace = {{ .to | quote }}
    {{- end }}
{{- end }}
EOF

{{- end }}

{{- if eq .runType "Normal" }}
  {{- range $host_name, $CA := .normal.moduleSourcesCA }}
    {{- if and (not (has $host_name $exist_registry_host_list)) $CA }}
      {{- $exist_registry_host_list = append $exist_registry_host_list $host_name }}

# Sync module sources host.toml and ca.crt
mkdir -p "/etc/containerd/registry.d/{{ $host_name }}"
bb-sync-file "/etc/containerd/registry.d/{{ $host_name }}/ca.crt" - << EOF
{{ $CA }}
EOF

bb-sync-file "/etc/containerd/registry.d/{{ $host_name }}/hosts.toml" - << EOF
server = {{ $host_name | quote }}
ca = ["/etc/containerd/registry.d/{{ $host_name }}/ca.crt"]
capabilities = ["pull", "resolve"]
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

# Update state
echo "$new_hosts" > "$hosts_state_file"

{{- end }}
