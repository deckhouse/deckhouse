{{- if ne .runType "ImageBuilding" }}
if bb-flag? is-bootstrapped; then exit 0; fi

units="kubelet.service"
for unit in $units; do
  systemctl enable "$unit"

  {{ if eq .runType "ClusterBootstrap" }}
  systemctl restart "$unit"
  {{ else }}
  if [[ ! -f /var/lib/bashible/reboot ]] ; then
    systemctl restart "$unit"
  fi
  {{- end }}
done
{{- end }}
