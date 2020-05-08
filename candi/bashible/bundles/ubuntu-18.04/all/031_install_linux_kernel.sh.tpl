{{- if ne .runType "ImageBuilding" }}
bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  bb-flag-set reboot
}
{{- end }}

desired_version="5.3.0-51-generic"
if [ -f /var/lib/bashible/kernel_version_desired_by_cloud_provider ]; then
  desired_version="$(</var/lib/bashible/kernel_version_desired_by_cloud_provider)"
fi

if (! bb-apt-package? "linux-image-${desired_version}") || (! bb-apt-package? "linux-modules-${desired_version}") || (! bb-apt-package? "linux-headers-${desired_version}"); then
  bb-deckhouse-get-disruptive-update-approval
  bb-apt-install "linux-image-${desired_version}" "linux-modules-${desired_version}" "linux-headers-${desired_version}"
  bb-apt-autoremove
fi

version_pattern="$(echo "$desired_version" | sed -r 's/([0-9\.-]+)-([^0-9]+)$/^linux-[a-z0-9\.-]+(\1|\1-\2)$/')"

packages="$(dpkg --get-selections | grep -E '^linux-.*\s(install|hold)$' | awk '{print $1}' | grep -Ev "$version_pattern" | grep -Ev '^linux-base$' || true)"
if [ -n "$packages" ]; then
  bb-apt-remove $packages
fi

rm -f /var/lib/bashible/kernel_version_desired_by_cloud_provider
