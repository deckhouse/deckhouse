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

# Avoid problems with expired ca-certificates
bb-apt-rpm-install --force ca-certificates

{{- if .registry.ca }}
bb-event-on 'registry-ca-changed' '_update_ca_certificates'
_update_ca_certificates() {
  bb-flag-set containerd-need-restart
  update-ca-certificates
}

bb-sync-file /usr/local/share/ca-certificates/registry-ca.crt - registry-ca-changed << "EOF"
{{ .registry.ca }}
EOF
{{- else }}
if [ -f /usr/local/share/ca-certificates/registry-ca.crt ]; then
  rm -f /usr/local/share/ca-certificates/registry-ca.crt
  _update_ca_certificates
fi
{{- end }}
