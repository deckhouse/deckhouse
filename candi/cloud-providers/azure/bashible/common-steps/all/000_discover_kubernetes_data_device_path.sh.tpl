{{- $nodeTypeList := list "CloudPermanent" }}
{{- if has .nodeGroup.nodeType $nodeTypeList }}
  {{- if eq .nodeGroup.name "master" }}

function discover_device_path() {
  local lun_name="$1"

  # Full device path via /dev/disk/azure/*/$lun_name
  local device_path="$(ls -1 /dev/disk/azure/*/$lun_name)"
  if [ "$(wc -l <<< "$device_path")" -ne 1 ]; then
    >&2 echo "Failed to discover device by lun: $lun_name"
    exit 1
  fi

  # Check if the symbolic link exists
  if [ ! -e "$device_path" ]; then
    >&2 echo "Failed to discover device: $device_path not found"
    exit 1
  fi
  
  # Resolve the symbolic link to the real path
  device_path=$(readlink -f "$device_path")

  # Check that the path is resolved and exists
  if [ -z "$device_path" ] || [ ! -b "$device_path" ]; then
    >&2 echo "Failed to resolve device path for: $lun_name"
    exit 1
  fi
  
  # Return the real device path
  echo "$device_path"
}

# Skip for
if [[ "$FIRST_BASHIBLE_RUN" != "yes" ]]; then
  exit 0
fi

# Skip for
if [ -f /var/lib/bashible/kubernetes-data-device-installed ]; then
  exit 0
fi

# Get Kubernetes data device
DATA_DEVICE=$(bb-get-kubernetes-data-device-from-file-or-secret)
if [ -z "$DATA_DEVICE" ]; then
  >&2 echo "failed to get kubernetes data device path"
  exit 1
fi

if [[ "$DATA_DEVICE" != /dev/* ]]; then
  DATA_DEVICE=$(discover_device_path "lun10")
  echo "kubernetes-data device: $DATA_DEVICE"
  echo "$DATA_DEVICE" > /var/lib/bashible/kubernetes_data_device_path
fi

  {{- end  }}
{{- end  }}
