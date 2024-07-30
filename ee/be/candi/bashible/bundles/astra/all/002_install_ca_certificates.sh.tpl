# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.
# Avoid problems with expired ca-certificates
bb-apt-install --force ca-certificates

# Hack for old distros
if grep -q "^mozilla\/DST_Root_CA_X3.crt$" /etc/ca-certificates.conf; then
  sed -i "/mozilla\/DST_Root_CA_X3.crt/d" /etc/ca-certificates.conf
  update-ca-certificates --fresh
fi

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
