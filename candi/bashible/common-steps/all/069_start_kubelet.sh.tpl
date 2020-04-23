if bb-flag? kubelet-need-restart; then
{{- if ne .runType "ImageBuilding" }}
  {{ if eq .runType "ClusterBootstrap" }}
  systemctl restart "kubelet.service"
  {{ else }}
  if ! bb-flag? reboot; then
    systemctl restart "kubelet.service"
  fi
  {{- end }}
{{- end }}

  bb-flag-unset kubelet-need-restart
fi
