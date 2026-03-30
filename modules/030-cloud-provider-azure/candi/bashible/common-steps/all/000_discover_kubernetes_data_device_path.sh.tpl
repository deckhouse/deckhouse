{{- if eq .nodeGroup.name "master" }}

if [[ "$FIRST_BASHIBLE_RUN" != "yes" ]]; then
  return 0
fi

if [ -f /var/lib/bashible/kubernetes-data-device-installed ]; then
  return 0
fi

if ! grep "/dev" /var/lib/bashible/kubernetes_data_device_path >/dev/null; then
 kubernetes_data_device_path=""

  # Method 1: Try standard Azure udev paths (SCSI and NVMe)
  for path in /dev/disk/azure/scsi1/lun10 /dev/disk/azure/data/by-lun/lun10; do
    if [ -L "$path" ]; then
      kubernetes_data_device_path="$(readlink -f "$path")"
      break
    fi
  done

  # Method 2: Fallback - search for unmounted empty disk with expected size
  # This handles cases where udev rules don't create /dev/disk/azure/ (e.g., NVMe without proper rules)
  if [ -z "$kubernetes_data_device_path" ]; then
    for disk in /dev/nvme*n[0-9] /dev/sd[b-z]; do
      [ -b "$disk" ] || continue

      # Skip if disk is mounted
      if mount | grep -q "^$disk"; then
        continue
      fi

      # Skip if disk has partitions
      if ls ${disk}p* ${disk}[0-9] 2>/dev/null | grep -q .; then
        continue
      fi

      # Skip if disk has filesystem
      if blkid "$disk" 2>/dev/null | grep -q TYPE; then
        continue
      fi

      # Check size: expect etcd data disk around 20GB (allow 15-50GB range)
      size_bytes=$(lsblk -b -n -o SIZE "$disk" 2>/dev/null || echo "0")
      size_gb=$((size_bytes / 1024 / 1024 / 1024))

      if [ "$size_gb" -ge 15 ] && [ "$size_gb" -le 50 ]; then
        kubernetes_data_device_path="$disk"
        >&2 echo "Found kubernetes-data device using fallback method: $disk (${size_gb}GB)"
        break
      fi
    done
  fi

  if [ -z "$kubernetes_data_device_path" ]; then
    >&2 echo "failed to discover kubernetes-data device"
    return 1
  fi
  
else
  return 0
fi

echo "kubernetes_data_device: $kubernetes_data_device_path"
blkid
echo "$kubernetes_data_device_path" > /var/lib/bashible/kubernetes_data_device_path
{{- end }}
