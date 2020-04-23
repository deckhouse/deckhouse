{{- if ne .runType "ClusterBootstrap" }}
if bb-flag? reboot; then
  bb-log-info "Reboot machine after bootstrap process completed"
  bb-flag-unset reboot
  (sleep 5; shutdown -r now) &
fi
{{- end }}
