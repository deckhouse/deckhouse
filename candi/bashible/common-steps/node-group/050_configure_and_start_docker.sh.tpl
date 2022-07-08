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

{{- if eq .cri "Docker" }}

bb-event-on 'docker-config-changed' '_on_docker_config_changed'
_on_docker_config_changed() {
{{ if ne .runType "ImageBuilding" -}}
  bb-deckhouse-get-disruptive-update-approval
  systemctl restart docker.service
{{- end }}
}

daemon_json="$(cat << "EOF"
{
{{- $max_concurrent_downloads := 3 }}
{{- if hasKey .nodeGroup.cri "docker" }}
  {{- $max_concurrent_downloads = .nodeGroup.cri.docker.maxConcurrentDownloads | default $max_concurrent_downloads }}
{{- end }}
        "log-driver": "json-file",
        "log-opts": {
                "max-file": "5",
                "max-size": "10m"
        },
	"max-concurrent-downloads": {{ $max_concurrent_downloads }}
{{- if eq .registry.scheme "http" }}
  "insecure-registries" : ["{{ .registry.address }}"]
{{- end }}
}
EOF
)"

if bb-is-ubuntu-version? 22.04 || bb-is-centos-version? 8 || bb-is-debian-version? 11; then
  daemon_json="$(jq '. + {"exec-opts": ["native.cgroupdriver=cgroupfs"]}' <<< "${daemon_json}")"
  daemon_json="${daemon_json_cgroupfs}"
fi

mkdir -p /etc/docker
bb-sync-file /etc/docker/daemon.json - docker-config-changed <<< ${daemon_json}

{{- if .registry.ca }}
mkdir -p /etc/docker/certs.d/{{ .registry.address }}
bb-sync-file /etc/docker/certs.d/{{ .registry.address }}/ca.crt  - << "EOF"
{{ .registry.ca }}
EOF
{{- end }}
{{- end }}

{{- if .registry.auth }}
if docker version >/dev/null 2>/dev/null; then
  username="$(base64 -d <<< "{{ .registry.auth }}" | awk -F ":" '{print $1}')"
  password="$(base64 -d <<< "{{ .registry.auth }}" | awk -F ":" '{print $2}')"
  HOME=/ docker login --username "${username}" --password "${password}" {{ .registry.address }}
fi
{{- end }}
