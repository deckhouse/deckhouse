{{- if eq .nodeGroup.name "master" }}
function get_data_device_secret() {
  secret="d8-masters-kubernetes-data-device-path"

  if [ -f /var/lib/bashible/bootstrap-token ]; then
    while true; do
      for server in {{ .normal.apiserverEndpoints | join " " }}; do
        if curl -s -f -X GET "https://$server/api/v1/namespaces/d8-system/secrets/$secret" --header "Authorization: Bearer $(</var/lib/bashible/bootstrap-token)" --cacert "$BOOTSTRAP_DIR/ca.crt"
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
  device_name="$(lsblk -Jo name,serial | jq -r --arg device_id "$cloud_disk_name" '.blockdevices[] | select(.serial==$device_id) | .name')"
  if [ "$device_name" == "" ]; then
    >&2 echo "failed to discover kubernetes-data device"
    return 1
  fi
  echo "/dev/$device_name"
}

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
