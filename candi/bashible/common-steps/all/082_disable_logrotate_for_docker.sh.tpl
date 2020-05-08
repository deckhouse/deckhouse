if [[ -f "/etc/logrotate.d/docker-containers" ]]; then
  rm -f /etc/logrotate.d/docker-containers
fi

if [[ -f "/etc/systemd/system/docker-logrotate.service" ]]; then
{{- if ne .runType "ImageBuilding" }}
  systemctl stop docker-logrotate.service
{{- end }}
  systemctl disable docker-logrotate.service
  rm -f /etc/systemd/system/docker-logrotate.service
{{- if ne .runType "ImageBuilding" }}
  systemctl daemon-reload
  systemctl reset-failed
{{- end }}
fi

if [[ -f "/etc/systemd/system/docker-logrotate.timer" ]]; then
{{- if ne .runType "ImageBuilding" }}
  systemctl stop docker-logrotate.timer
{{- end }}
  systemctl disable docker-logrotate.timer
  rm -f /etc/systemd/system/docker-logrotate.timer
fi
