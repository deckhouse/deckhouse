if [[ -d /var/lib/cni/networks/cbr0 ]]; then
  bb-deckhouse-get-disruptive-update-approval
  systemctl stop kubelet.service
  rm -rf /var/lib/cni/networks/cbr0
  bb-log-info "Removed /var/lib/cni/networks/cbr0"
  bb-flag-set reboot
fi
