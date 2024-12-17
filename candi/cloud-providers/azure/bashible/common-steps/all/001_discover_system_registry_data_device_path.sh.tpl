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

# The system registry file is always created in step 000_create_system_registry_data_device_path.sh.tpl
system_registry_file="/var/lib/bashible/system_registry_data_device_path"

# Get system registry data device
DATA_DEVICE=$(cat "$system_registry_file")

if [ -n "$DATA_DEVICE" ] && [[ "$DATA_DEVICE" != /dev/* ]]; then
  DATA_DEVICE=$(discover_device_path "lun11")
  echo "system-registry-data device: $DATA_DEVICE"
  echo "$DATA_DEVICE" > "$system_registry_file"
fi

  {{- end  }}
{{- end  }}
