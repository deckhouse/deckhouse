{{- $nodeTypeList := list "CloudPermanent" }}
{{- if has .nodeGroup.nodeType $nodeTypeList }}
  {{- if eq .nodeGroup.name "master" }}
    {{- if and .registry.registryMode (ne .registry.registryMode "Direct") }}

if [[ "$FIRST_BASHIBLE_RUN" != "yes" ]]; then
  return 0
fi

if [ -f /var/lib/bashible/system-registry-data-device-installed ]; then
  return 0
fi

if ! grep "/dev" /var/lib/bashible/system_registry_data_device_path >/dev/null; then
  get_disks_by_lun_id="$(ls /dev/disk/azure/*/lun11 -l)"

  if [ "$(wc -l <<< "$get_disks_by_lun_id")" -ne 1 ]; then
    >&2 echo "failed to discover system-registry-data device"
    return 1
  fi

  system_registry_data_device_path="$(awk '{gsub("../../..", "/dev");print $11}' <<< "$get_disks_by_lun_id")"
else
  return 0
fi

echo "system_registry_data_device: $system_registry_data_device_path"
blkid
echo "$system_registry_data_device_path" > /var/lib/bashible/system_registry_data_device_path

    {{- end  }}
  {{- end  }}
{{- end  }}
