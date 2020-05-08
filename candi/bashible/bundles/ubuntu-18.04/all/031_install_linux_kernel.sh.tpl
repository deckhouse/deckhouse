{{- if ne .runType "ImageBuilding" }}
bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  bb-flag-set reboot
}
{{- end }}

desired_version="5.3.0-51-generic"
if (! bb-apt-package? "linux-image-${desired_version}") || (! bb-apt-package? "linux-modules-${desired_version}") || (! bb-apt-package? "linux-headers-${desired_version}"); then
  bb-deckhouse-get-disruptive-update-approval
  bb-apt-install "linux-image-${desired_version}" "linux-modules-${desired_version}" "linux-headers-${desired_version}"
  bb-apt-autoremove
fi

desired_version_common="$(echo "$desired_version" | sed -r 's/-[^0-9]+$//')"
packages_image="$(  dpkg -l 'linux-image-*'   | grep '^[a-z]i' | grep -Fv "$desired_version_common" || true)"
packages_headers="$(dpkg -l 'linux-headers-*' | grep '^[a-z]i' | grep -Fv "$desired_version_common" || true)"
packages_modules="$(dpkg -l 'linux-modules-*' | grep '^[a-z]i' | grep -Fv "$desired_version_common" || true)"

for pkg in $packages_image $packages_headers $packages_modules; do
  bb-apt-remove $pkg
done
