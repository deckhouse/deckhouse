{{- if eq .nodeGroup.name "master" }}

if [[ "$FIRST_BASHIBLE_RUN" != "yes" ]]; then
  return 0
fi

if [ -f /var/lib/bashible/kubernetes-data-device-installed ]; then
  return 0
fi

if ! grep "/dev" /var/lib/bashible/kubernetes_data_device_path >/dev/null; then
  get_disks_by_lun_id="$(ls /dev/disk/azure/*/lun10 -l)"

  if [ "$(wc -l <<< "$get_disks_by_lun_id")" -ne 1 ]; then
    >&2 echo "failed to discover kubernetes-data device"
    return 1
  fi

  kubernetes_data_device_path="$(awk '{gsub("../../..", "/dev");print $11}' <<< "$get_disks_by_lun_id")"
else
  return 0
fi

echo "kubernetes_data_device: $kubernetes_data_device_path"
blkid
echo "$kubernetes_data_device_path" > /var/lib/bashible/kubernetes_data_device_path
{{- end }}
