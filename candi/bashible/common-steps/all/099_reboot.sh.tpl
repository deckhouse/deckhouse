{{- if ne .runType "ClusterBootstrap" }}
if bb-flag? reboot; then
  bb-deckhouse-get-disruptive-update-approval
  bb-log-info "Rebooting machine after bootstrap process completed"
  bb-flag-unset reboot
  shutdown -r now
fi
{{- else }}
bb-flag-unset reboot
{{- end }}
