# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

bb-apt-install --force ca-certificates

# Hack for Astra 2.12
if bb-is-astra-version? 2.12.+ ; then
  if grep -q "^mozilla\/DST_Root_CA_X3.crt$" /etc/ca-certificates.conf; then
    sed -i "/mozilla\/DST_Root_CA_X3.crt/d" /etc/ca-certificates.conf
    update-ca-certificates --fresh
  fi
fi

{{- if .registry.ca }}
bb-event-on 'registry-ca-changed' '_update_ca_certificates'
function _update_ca_certificates() {
  update-ca-certificates
}

bb-sync-file /usr/local/share/ca-certificates/registry-ca.crt - registry-ca-changed << "EOF"
{{ .registry.ca }}
EOF
{{- end }}
