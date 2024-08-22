# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

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
