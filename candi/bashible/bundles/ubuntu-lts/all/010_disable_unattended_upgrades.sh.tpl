# Disable auto reboot and remove unused deps
if [ -f "/etc/apt/apt.conf.d/50unattended-upgrades" ] ; then
  sed -i 's/\/\/Unattended-Upgrade::Automatic-Reboot "false"/Unattended-Upgrade::Automatic-Reboot "false"/g' /etc/apt/apt.conf.d/50unattended-upgrades
  sed -i 's/\/\/Unattended-Upgrade::InstallOnShutdown "true"/Unattended-Upgrade::InstallOnShutdown "false"/g' /etc/apt/apt.conf.d/50unattended-upgrades
  sed -i 's/\/\/Unattended-Upgrade::Remove-Unused-Dependencies "false"/Unattended-Upgrade::Remove-Unused-Dependencies "false"/g' /etc/apt/apt.conf.d/50unattended-upgrades
fi
