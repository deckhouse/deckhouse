{{ if ne .runType "ImageBuilding" -}}
bb-event-on 'bb-sync-file-changed' '_on_rsyslog_config_changed'
_on_rsyslog_config_changed() {
  systemctl restart rsyslog
}
{{- end }}

bb-sync-file /etc/rsyslog.d/10-kubelet.conf - <<END
:programname,isequal, "kubelet" ~
END

bb-sync-file /etc/rsyslog.d/10-dockerd.conf - <<END
:programname,isequal, "dockerd" ~
END
