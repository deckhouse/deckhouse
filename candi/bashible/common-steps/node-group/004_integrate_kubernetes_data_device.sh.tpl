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

{{- $nodeTypeList := list "CloudEphemeral" "CloudPermanent" "CloudStatic" }}
{{- if has .nodeGroup.nodeType $nodeTypeList }}
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

if [[ "$FIRST_BASHIBLE_RUN" != "yes" ]]; then
  exit 0
fi

if [ -f /var/lib/bashible/kubernetes-data-device-installed ]; then
  exit 0
fi

if [ -f /var/lib/bashible/kubernetes_data_device_path ]; then
  DATA_DEVICE="$(</var/lib/bashible/kubernetes_data_device_path)"
else
  DATA_DEVICE="$(get_data_device_secret | jq -re --arg hostname "$HOSTNAME" '.data[$hostname]' | base64 -d)"
fi

mkdir -p /mnt/kubernetes-data

if ! file -s $DATA_DEVICE | grep -q ext4; then
  mkfs.ext4 -F -L kubernetes-data $DATA_DEVICE
fi

if grep -qv kubernetes-data /etc/fstab; then
  cat >> /etc/fstab << EOF
LABEL=kubernetes-data           /mnt/kubernetes-data     ext4   defaults,discard,x-systemd.automount        0 0
EOF
fi

if ! mount | grep -q $DATA_DEVICE; then
  mount -L kubernetes-data
fi

mkdir -p /mnt/kubernetes-data/var-lib-etcd
chmod 700 /mnt/kubernetes-data/var-lib-etcd
mkdir -p /mnt/kubernetes-data/etc-kubernetes

# if there is kubernetes dir with regular files then we can't delete it
# if there aren't files then we can delete dir to prevent symlink creation problems
if [[ "$(find /etc/kubernetes/ -type f 2>/dev/null | wc -l)" == "0" ]]; then
  rm -rf /etc/kubernetes
  ln -s /mnt/kubernetes-data/etc-kubernetes /etc/kubernetes
fi

if [[ "$(find /var/lib/etcd/ -type f 2>/dev/null | wc -l)" == "0" ]]; then
  rm -rf /var/lib/etcd
  ln -s /mnt/kubernetes-data/var-lib-etcd /var/lib/etcd
fi

touch /var/lib/bashible/kubernetes-data-device-installed
  {{- end  }}
{{- end  }}
