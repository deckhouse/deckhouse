{{- if ne .runType "ImageBuilding" }}
bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  bb-flag-set reboot
}
{{- end }}

kernel_version="5.3.0.51.104"
bb-apt-install linux-generic-hwe-18.04="$kernel_version"

bb-apt-autoremove
