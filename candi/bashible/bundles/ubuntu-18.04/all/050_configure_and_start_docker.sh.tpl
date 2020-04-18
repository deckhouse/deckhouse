if bb-flag? is-bootstrapped; then exit 0; fi

cat << "EOF" > /etc/docker/daemon.json
{
        "log-driver": "json-file",
        "log-opts": {
                "max-file": "5",
                "max-size": "10m"
        }
}
EOF

{{ if ne .runType "ImageBuilding" -}}
units="docker.service"

for unit in $units; do
  systemctl enable "$unit" && systemctl restart "$unit"
done
{{- end }}
