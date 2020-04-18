{{- if eq .runType "ImageBuilding" }}
if bb-flag? is-bootstrapped; then exit 0; fi

export DEBIAN_FRONTEND=noninteractive
apt -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" dist-upgrade -y
{{- end }}
