{{- $nodeTypeList := list "CloudPermanent" }}
{{- if has .nodeGroup.nodeType $nodeTypeList }}
  {{- if eq .nodeGroup.name "master" }}

# The file always exists (created in step 000_create_system_registry_data_device_path.sh.tpl)
system_registry_file="/var/lib/bashible/system_registry_data_device_path"

# Read the device path from the file
dataDevice=$(cat "$system_registry_file")

# If $dataDevice is empty, exit the script
if [ -z "$dataDevice" ]; then
  # Nothing to do
  exit 0
fi

# If dataDevice is non-empty and begins with /dev, log it and exit
if [[ "$dataDevice" == /dev/* ]]; then
  # Nothing to do
  echo "system_registry_data_device: $dataDevice"
  exit 0
fi

# Attempt to list devices at a specific LUN path
get_disks_by_lun_id="$(ls /dev/disk/azure/*/lun11 -l 2>/dev/null)"

# Check if the result is empty
if [ -z "$get_disks_by_lun_id" ]; then
  # If no devices are found, clear the file
  : > "$system_registry_file"
else
  # Ensure only one device is found; otherwise, report failure
  if [ "$(wc -l <<< "$get_disks_by_lun_id")" -ne 1 ]; then
    >&2 echo "Failed to discover system-registry-data device"
    exit 1
  fi

  # Extract the device path from the listing
  new_device_path="$(awk '{gsub("../../..", "/dev"); print $11}' <<< "$get_disks_by_lun_id")"

  # Log the discovered device path and write it to the file
  echo "system_registry_data_device: $new_device_path"
  echo "$new_device_path" > "$system_registry_file"
fi

# List block devices for diagnostic purposes
blkid

  {{- end  }}
{{- end  }}
