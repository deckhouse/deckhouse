if systemctl is-enabled -q firewalld || systemctl is-active -q firewalld; then
  systemctl stop firewalld
  systemctl disable firewalld
  systemctl mask firewalld
fi
