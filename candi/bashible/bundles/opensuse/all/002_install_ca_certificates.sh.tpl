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

bb-zypper-install ca-certificates
# Hack to avoid problems with certs in d8-curl and possible with alpine busybox for kube-apiserver
if [[ ! -e /etc/ssl/certs/ca-certificates.crt ]]; then
  mkdir -p /etc/ssl/certs
  ln -s /var/lib/ca-certificates/ca-bundle.pem /etc/ssl/certs/ca-certificates.crt
fi

{{- if .registry.ca }}
bb-event-on 'registry-ca-changed' '_update_ca_certificates'
_update_ca_certificates() {
  bb-flag-set containerd-need-restart
  update-ca-certificates
}

bb-sync-file /var/lib/ca-certificates/pem/registry-ca.pem - registry-ca-changed << "EOF"
{{ .registry.ca }}
EOF
{{- else }}
if [ -f /var/lib/ca-certificates/pem/registry-ca.pem ]; then
  rm -f /var/lib/ca-certificates/pem/registry-ca.pem
  _update_ca_certificates
fi
{{- end }}
