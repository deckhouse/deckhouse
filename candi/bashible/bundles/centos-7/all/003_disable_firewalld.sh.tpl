if systemctl is-active -q firewalld; then
  systemctl stop firewalld
fi

if systemctl is-enabled -q firewalld; then
  systemctl disable firewalld
  systemctl mask firewalld
fi
