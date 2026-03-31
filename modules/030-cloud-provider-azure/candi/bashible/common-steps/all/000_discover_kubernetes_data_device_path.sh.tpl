{{- if eq .nodeGroup.name "master" }}

# Terraform-only deterministic mode (no autodiscovery)

kubernetes_data_device_id="$(cat /var/lib/bashible/kubernetes_data_device_path 2>/dev/null || true)"

if [[ "$FIRST_BASHIBLE_RUN" != "yes" ]]; then
  return 0
fi

if [ -z "$kubernetes_data_device_id" ]; then
  >&2 echo "kubernetes_data_device_path is not set. Provide it via Terraform/cloud-init."
  return 1
fi

kubernetes_data_device_path=""

# Direct block device path (NVMe / by-id)
if [ -b "$kubernetes_data_device_id" ]; then
  kubernetes_data_device_path="$kubernetes_data_device_id"

# Azure SCSI / udev-based LUN mapping (explicit only)
elif [[ "$kubernetes_data_device_id" == lun* ]]; then
  lun_number="${kubernetes_data_device_id#lun}"

  kubernetes_data_device_path="$(ls -1 /dev/disk/azure/data-lun${lun_number} 2>/dev/null | head -n1 || true)"

  if [ -z "$kubernetes_data_device_path" ]; then
    >&2 echo "Azure disk for $kubernetes_data_device_id not found (/dev/disk/azure/data-lun${lun_number})"
    return 1
  fi

else
  >&2 echo "Unsupported kubernetes_data_device_path format: $kubernetes_data_device_id"
  return 1
fi

echo "kubernetes_data_device: $kubernetes_data_device_path"
blkid
{{- end }}