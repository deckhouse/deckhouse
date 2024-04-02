# Copyright 2023 Flant JSC
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
# Hack to avoid problems with certs in d8-curl and possible with alpine busybox for kube-apiserver
if [[ ! -e /etc/ssl/certs/ca-certificates.crt ]]; then
  mkdir -p /etc/ssl
  pushd /etc/ssl >/dev/null
  ln -s ../pki/tls/certs /etc/ssl/certs
  popd > /dev/null
  ln -s /etc/ssl/certs/ca-bundle.crt /etc/ssl/certs/ca-certificates.crt
fi

{{- if .registry.ca }}
bb-event-on 'registry-ca-changed' '_update_ca_certificates'
_update_ca_certificates() {
  bb-flag-set containerd-need-restart
  update-ca-trust
}

bb-sync-file /etc/pki/ca-trust/source/anchors/registry-ca.crt - registry-ca-changed << "EOF"
{{ .registry.ca }}
EOF
{{- else }}
if [ -f /etc/pki/ca-trust/source/anchors/registry-ca.crt ]; then
  rm -f /etc/pki/ca-trust/source/anchors/registry-ca.crt
  _update_ca_certificates
fi
{{- end }}
