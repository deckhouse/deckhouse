if systemctl list-units | grep -q rpcbind.socket; then
  systemctl stop rpcbind.socket
  systemctl disable rpcbind.socket
fi

if systemctl list-units | grep -q rpcbind.service; then
  systemctl stop rpcbind.service
  systemctl disable rpcbind.service
fi
