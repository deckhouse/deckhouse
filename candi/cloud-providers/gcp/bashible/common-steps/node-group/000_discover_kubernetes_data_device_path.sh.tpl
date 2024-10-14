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

function discover_device_path() {
  cloud_disk_name="$1"
  device_name="$(lsblk -lo name,serial | grep "$cloud_disk_name" | cut -d " " -f1)"
  if [ "$device_name" == "" ]; then
    >&2 echo "failed to discover kubernetes-data device"
    return 1
  fi
  echo "/dev/$device_name"
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
  cloud_disk_name="$(get_data_device_secret | jq -re --arg hostname "$HOSTNAME" '.data[$hostname]' | base64 -d)"
fi

echo "$(discover_device_path "$cloud_disk_name")" > /var/lib/bashible/kubernetes_data_device_path
{{- end }}
