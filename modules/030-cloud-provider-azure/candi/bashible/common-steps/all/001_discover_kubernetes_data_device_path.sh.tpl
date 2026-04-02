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
elif [[ "$kubernetes_data_device_id" == lun* ]] || [[ "$kubernetes_data_device_id" =~ ^[0-9]+$ ]]; then
  # Extract LUN number: accept "lun10" or just "10"
  if [[ "$kubernetes_data_device_id" == lun* ]]; then
    lun_number="${kubernetes_data_device_id#lun}"
  else
    lun_number="$kubernetes_data_device_id"
  fi

  # Method 1: Try new Azure udev rules (works on both SCSI and NVMe VMs with 80-azure-disk.rules)
  kubernetes_data_device_path="$(ls -1 /dev/disk/azure/data/by-lun/${lun_number} 2>/dev/null | head -n1 || true)"

  # Method 2: Try legacy Azure SCSI udev path (works on older SCSI VMs)
  if [ -z "$kubernetes_data_device_path" ]; then
    kubernetes_data_device_path="$(ls -1 /dev/disk/azure/data-lun${lun_number} /dev/disk/azure/scsi*/lun${lun_number} 2>/dev/null | head -n1 || true)"
  fi

  # Method 3: Try NVMe by-path fallback (works on NVMe VMs without udev rules)
  # Azure NVMe namespace mapping: LUN N typically maps to namespace N+2 (e.g., LUN 10 → ns 12)
  if [ -z "$kubernetes_data_device_path" ]; then
    # Try common namespace patterns: LUN+2, LUN+1, LUN itself
    for ns_offset in 2 1 0; do
      ns_id=$((lun_number + ns_offset))
      path_candidate="$(ls -1 /dev/disk/by-path/*nvme-${ns_id} 2>/dev/null | head -n1 || true)"
      if [ -n "$path_candidate" ] && [ -b "$path_candidate" ]; then
        kubernetes_data_device_path="$path_candidate"
        break
      fi
    done
  fi

  if [ -z "$kubernetes_data_device_path" ]; then
    >&2 echo "Azure disk for $kubernetes_data_device_id (LUN ${lun_number}) not found"
    >&2 echo "Tried: /dev/disk/azure/data/by-lun/${lun_number}, /dev/disk/azure/*lun${lun_number}, /dev/disk/by-path/*nvme-*"
    return 1
  fi

else
  >&2 echo "Unsupported kubernetes_data_device_path format: $kubernetes_data_device_id"
  return 1
fi

echo "kubernetes_data_device: $kubernetes_data_device_path"
blkid
{{- end }}