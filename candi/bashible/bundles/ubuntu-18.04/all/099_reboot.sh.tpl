if [[ -f "/var/lib/bashible/reboot" ]]; then
  echo "Reboot machine after bootstrap process completed"
  rm -f /var/lib/bashible/reboot
  (sleep 5; shutdown -r now) &
fi
