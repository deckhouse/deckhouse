# Copyright 2021 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

{{- if eq .nodeGroup.name "master" }}

function get_data_device_secret() {
  secret="d8-masters-kubernetes-data-device-path"

  if [ -f /var/lib/bashible/bootstrap-token ]; then
    while true; do
      for server in {{ .normal.apiserverEndpoints | join " " }}; do
        if d8-curl --connect-timeout 10 -s -f -X GET \
          "https://$server/api/v1/namespaces/d8-system/secrets/$secret" \
          --header "Authorization: Bearer $(</var/lib/bashible/bootstrap-token)" \
          --cacert "$BOOTSTRAP_DIR/ca.crt"
        then
          return 0
        else
          >&2 echo "failed to get secret $secret from server $server"
        fi
      done
      sleep 10
    done
  else
    >&2 echo "failed to get secret $secret: can't find bootstrap-token"
    return 1
  fi
}

function discover_device_path() {
  cloud_disk_name="$1"

  # 1) Prefer stable by-id links (BEST PRACTICE)
  if [ -n "$cloud_disk_name" ] && [ -d /dev/disk/by-id ]; then
    byid_match="$(ls -1 /dev/disk/by-id/ 2>/dev/null | grep -F "$cloud_disk_name" | head -n1)"
    if [ -n "$byid_match" ] && [ -e "/dev/disk/by-id/$byid_match" ]; then
      echo "$(readlink -f "/dev/disk/by-id/$byid_match")"
      return 0
    fi
  fi

  # 2) Safe SERIAL fallback (strict match)
  device_name="$(lsblk -dn -o NAME,SERIAL 2>/dev/null | awk -v id="$cloud_disk_name" '$2==id {print $1; exit}')"
  if [ -n "$device_name" ]; then
    echo "/dev/$device_name"
    return 0
  fi

  # 3) Azure fallback (robust LUN scan)
  if [ -d /dev/disk/azure ]; then
    azure_device="$(readlink -f /dev/disk/azure/scsi*/* 2>/dev/null | head -n1)"
    if [ -n "$azure_device" ]; then
      echo "$azure_device"
      return 0
    fi
  fi

  return 1
}

if [[ "$FIRST_BASHIBLE_RUN" != "yes" ]]; then
  return 0
fi

if [ -f /var/lib/bashible/kubernetes-data-device-installed ]; then
  return 0
fi

if [ -f /var/lib/bashible/kubernetes_data_device_path ]; then
  if ! grep "/dev" /var/lib/bashible/kubernetes_data_device_path >/dev/null; then
    cloud_disk_name="$(cat /var/lib/bashible/kubernetes_data_device_path)"
  else
    return 0
  fi
else
  cloud_disk_name="$(get_data_device_secret | jq -re --arg hostname "$(bb-d8-node-name)" '.data[$hostname]' | base64 -d)"
fi

>&2 echo "discover_device_path: resolving disk for: $cloud_disk_name"

# Robust root disk detection (works with LVM, mapper, NVMe, cloud images)
root_src="$(findmnt -n -o SOURCE / 2>/dev/null)"
root_dev="$(lsblk -no PKNAME "$root_src" 2>/dev/null)"

# fallback if PKNAME fails (LVM/mapper cases)
if [ -z "$root_dev" ]; then
  root_dev="$(lsblk -ln -o NAME,MOUNTPOINT 2>/dev/null | awk '$2=="/" {print $1; exit}')"
fi

# build candidate list safely (exclude system disks early)
candidates="$(lsblk -dn -o NAME,TYPE 2>/dev/null | awk '$2=="disk"{print "/dev/"$1}')"

for disk in $candidates; do
  [ -e "$disk" ] || continue

  # Skip root disk completely
  base_disk="$(basename "$disk" 2>/dev/null | sed 's/p[0-9]*$//')"
  if [ -n "$root_dev" ] && [ "$base_disk" = "$root_dev" ]; then
    continue
  fi

  # skip disks with any mountpoints
  if lsblk -no MOUNTPOINT "$disk" 2>/dev/null | grep -q "/"; then
    continue
  fi

  # skip OS-like partitions
  part_types="$(lsblk -no PARTTYPE "$disk" 2>/dev/null | tr '\n' ' ')"
  if echo "$part_types" | grep -Eqi "efi|swap|linux_raid|linux_lvm"; then
    continue
  fi

  echo "$disk"
  return 0
done

>&2 echo "FAILED: no safe data disk found (system disks excluded)"
return 1

echo "$(discover_device_path "$cloud_disk_name")" > /var/lib/bashible/kubernetes_data_device_path

{{- end }}