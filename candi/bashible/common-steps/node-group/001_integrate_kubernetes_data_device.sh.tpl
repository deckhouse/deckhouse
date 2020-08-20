{{- if or (eq .nodeGroup.nodeType "Cloud") (eq .nodeGroup.nodeType "Hybrid") }}
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
mkdir -p /var/lib/etcd
mkdir -p /etc/kubernetes

if ! file -s $DATA_DEVICE | grep -q ext4; then
  mkfs.ext4 -L kubernetes-data $DATA_DEVICE
fi

if grep -qv kubernetes-data /etc/fstab; then
  cat >> /etc/fstab << EOF
LABEL=kubernetes-data           /mnt/kubernetes-data     ext4   defaults,discard        0 0
/mnt/kubernetes-data/var-lib-etcd       /var/lib/etcd   none    bind    0 0
/mnt/kubernetes-data/etc-kubernetes     /etc/kubernetes none    bind    0 0
EOF
fi

if ! mount | grep -q $DATA_DEVICE; then
  mount -L kubernetes-data
fi

mkdir -p /mnt/kubernetes-data/var-lib-etcd
mkdir -p /mnt/kubernetes-data/etc-kubernetes

mount -a

touch /var/lib/bashible/kubernetes-data-device-installed
  {{- end  }}
{{- end  }}
