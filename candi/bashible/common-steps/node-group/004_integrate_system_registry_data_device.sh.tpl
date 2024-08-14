# Copyright 2024 Flant JSC
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

{{- if and .registry.registryMode (ne .registry.registryMode "Direct") }}
{{- $nodeTypeList := list "CloudEphemeral" "CloudPermanent" "CloudStatic" }}

  {{- if has .nodeGroup.nodeType $nodeTypeList }}
    {{- if eq .nodeGroup.name "master" }}
function get_data_device_secret() {
  secret="d8-masters-system-registry-data-device-path"

  if [ -f /var/lib/bashible/bootstrap-token ]; then
    while true; do
      for server in {{ .normal.apiserverEndpoints | join " " }}; do
        if d8-curl -s -f -X GET "https://$server/api/v1/namespaces/d8-system/secrets/$secret" --header "Authorization: Bearer $(</var/lib/bashible/bootstrap-token)" --cacert "$BOOTSTRAP_DIR/ca.crt"
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

if [[ "$FIRST_BASHIBLE_RUN" != "yes" ]]; then
  exit 0
fi

if [ -f /var/lib/bashible/system-registry-data-device-installed ]; then
  exit 0
fi

if [ -f /var/lib/bashible/system_registry_data_device_path ]; then
  DATA_DEVICE="$(</var/lib/bashible/system_registry_data_device_path)"
else
  DATA_DEVICE="$(get_data_device_secret | jq -re --arg hostname "$HOSTNAME" '.data[$hostname]' | base64 -d)"
fi

{{- /*
# Sometimes the `device_path` output from terraform points to a non-existent device.
# In such situation we want to find an unpartitioned unused disk
# with no file system, assuming it is the correct one.
#
# Example of this situation (lsblk -o path,type,mountpoint,fstype --tree --json):
#         {
#          "path": "/dev/vda",
#          "type": "disk",
#          "mountpoint": null,
#          "fstype": null,
#          "children": [
#             {
#                "path": "/dev/vda1",
#                "type": "part",
#                "mountpoint": "/",
#                "fstype": "ext4"
#             },{
#                "path": "/dev/vda15",
#                "type": "part",
#                "mountpoint": "/boot/efi",
#                "fstype": "vfat"
#             }
#          ]
#       },{
#          "path": "/dev/vdb",
#          "type": "disk",
#          "mountpoint": null,
#          "fstype": null
#       }
*/}}
if ! [ -b "$DATA_DEVICE" ]; then
  >&2 echo "failed to find $DATA_DEVICE disk. Trying to detect the correct one"
  DATA_DEVICE=$(lsblk -o path,type,mountpoint,fstype --tree --json | jq -r '.blockdevices[] | select (.type == "disk" and .mountpoint == null and .children == null) | .path')
fi

    {{- end  }}
  {{- end  }}
{{- end }}
