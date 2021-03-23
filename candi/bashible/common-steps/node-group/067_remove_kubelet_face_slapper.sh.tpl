# TODO remove this step on next release
if systemctl is-enabled kubelet-face-slapper.timer >/dev/null 2>/dev/null; then
  systemctl stop kubelet-face-slapper.timer
  systemctl disable kubelet-face-slapper.timer
  rm -f /var/lib/bashible/kubelet-face-slapper.sh /etc/systemd/system/kubelet-face-slapper.service /etc/systemd/system/kubelet-face-slapper.timer
  systemctl daemon-reload
fi
