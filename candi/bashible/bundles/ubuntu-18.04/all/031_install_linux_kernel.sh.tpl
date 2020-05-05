{{- if ne .runType "ImageBuilding" }}
bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  bb-flag-set reboot
}
{{- end }}

version="5.3.0-51-generic"

if (! bb-apt-package? "linux-image-${version}") || (! bb-apt-package? "linux-modules-${version}") || (! bb-apt-package? "linux-headers-${version}"); then
  bb-deckhouse-get-disruptive-update-approval
  bb-apt-install "linux-image-${version}" "linux-modules-${version}" "linux-headers-${version}"
  bb-apt-autoremove
fi
