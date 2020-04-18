changed=no

if [[ ! -f "/etc/rsyslog.d/10-kubelet.conf" ]]; then
  cat > /etc/rsyslog.d/10-kubelet.conf <<END
:programname,isequal, "kubelet" ~
END
  changed=yes
fi

if [[ ! -f "/etc/rsyslog.d/10-dockerd.conf" ]]; then
  cat > /etc/rsyslog.d/10-dockerd.conf <<END
:programname,isequal, "dockerd" ~
END
  changed=yes
fi

{{ if ne .runType "ImageBuilding" -}}
if [[ "$changed" == "yes" ]]; then
  systemctl restart rsyslog
fi
{{- end }}
