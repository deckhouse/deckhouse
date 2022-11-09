# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

{{- $manage_kernel := true }}
{{- if hasKey .nodeGroup "operatingSystem" }}
  {{- if not .nodeGroup.operatingSystem.manageKernel }}
    {{- $manage_kernel = false }}
  {{- end }}
{{- end }}

{{- if $manage_kernel }}
{{- if ne .runType "ImageBuilding" }}
bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  bb-log-info "Setting reboot flag due to kernel was updated"
  bb-flag-set reboot
}
{{- end }}

metapackages="$(
  dpkg --get-selections | grep -E '^(linux|linux-image|linux-headers)-(aws|azure|gcp|generic|gke|kvm|lowlatency|oem|oracle|virtual)\s+(install|hold)' | awk '{print $1}' || true
)"
if [ -n "$metapackages" ]; then
  bb-apt-remove $metapackages
fi

{{- range $key, $value := index .k8s .kubernetesVersion "bashible" "astra" }}
  {{- $astraVersion := toString $key }}
  {{- if $value.kernel.generic }}
    {{- if or $value.kernel.generic.desiredVersion $value.kernel.generic.allowedPattern }}
if bb-is-astra-version? {{ $astraVersion }} ; then
  desired_version={{ $value.kernel.generic.desiredVersion | quote }}
  allowed_versions_pattern={{ $value.kernel.generic.allowedPattern | quote }}
fi
    {{- end }}
  {{- end }}
{{- end }}

if [ -f /var/lib/bashible/kernel_version_config_by_cloud_provider ]; then
  source /var/lib/bashible/kernel_version_config_by_cloud_provider
fi

if [[ -z $desired_version ]]; then
  bb-log-error "Desired version must be set"
  exit 1
fi

should_install_kernel=true
version_in_use="$(uname -r)"
if test -n "$allowed_versions_pattern" && grep -Eq "$allowed_versions_pattern" <<< "$version_in_use"; then
  should_install_kernel=false
fi

if [[ "$version_in_use" == "$desired_version" ]]; then
  should_install_kernel=false
fi

# Example: "5.4.0-54-generic" -> "^linux-[a-z0-9.-]+(5.4.0-54|5.4.0-54-generic)$"
desired_version_pattern="$(echo "$desired_version" | sed -r 's/([0-9\.-]+)-([^0-9]+)$/^linux-[a-z0-9\.-]+(\1|\1-\2)$/')"
version_in_use_pattern="$(echo "$version_in_use" | sed -r 's/([0-9\.-]+)-([^0-9]+)$/^linux-[a-z0-9\.-]+(\1|\1-\2)$/')"

if [[ "$should_install_kernel" == true ]]; then
  bb-deckhouse-get-disruptive-update-approval
  bb-apt-install "linux-image-${desired_version}" "linux-modules-${desired_version}" "linux-modules-extra-${desired_version}" "linux-headers-${desired_version}"
  packages_to_remove="$(
    dpkg --get-selections | grep -E '^linux-.*\s(install|hold)$' | awk '{print $1}' | grep -Ev "$desired_version_pattern" | grep -Ev 'linux-[^0-9]+$' || true
  )"
else
  packages_to_remove="$(
    dpkg --get-selections | grep -E '^linux-.*\s(install|hold)$' | awk '{print $1}' | grep -Ev "$version_in_use_pattern" | grep -Ev 'linux-[^0-9]+$' || true
  )"
fi

if [ -n "$packages_to_remove" ]; then
  bb-apt-remove $packages_to_remove
fi

rm -f /var/lib/bashible/kernel_version_config_by_cloud_provider
{{- end }}
