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

{{ $kubeadmDir := "/var/lib/bashible/kubeadm/v1beta4" }}

check_container_running() {
  local container_name=$1
  local max_retries=20
  local sleep_interval=10
  local count=0

  while [[ $count -lt $max_retries ]]; do
    if crictl ps -o json | jq -e --arg name "$container_name" '.containers[] | select(.metadata.name == $name and .state == "CONTAINER_RUNNING")' > /dev/null; then
      echo "$container_name is running"
      return 0
    fi
    count=$((count + 1))

    if [[ $count -ge $max_retries ]]; then
      echo "$container_name not running in $sleep_interval*$max_retries"
      exit 1
    fi

    sleep $sleep_interval
    echo "wait for the $container_name to start $count"
  done
}

check_container_running "kubernetes-api-proxy"
mkdir -p /etc/kubernetes/deckhouse/kubeadm/patches/
cp {{ $kubeadmDir}}/patches/* /etc/kubernetes/deckhouse/kubeadm/patches/
kubeadm init phase certs all --config {{ $kubeadmDir}}/config.yaml
kubeadm init phase kubeconfig all --config {{ $kubeadmDir}}/config.yaml
kubeadm init phase etcd local --config {{ $kubeadmDir}}/config.yaml
check_container_running "etcd"

mkdir -p /etc/kubernetes/deckhouse/extra-files
bb-sync-file /etc/kubernetes/deckhouse/extra-files/authentication-config.yaml - << EOF
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
anonymous:
  enabled: true
  conditions:
  - path: /livez
  - path: /readyz
  - path: /healthz
EOF

kubeadm init phase control-plane all --config {{ $kubeadmDir}}/config.yaml
check_container_running "kube-apiserver"
check_container_running "kube-controller-manager"
check_container_running "kube-scheduler"
kubeadm init phase mark-control-plane --config {{ $kubeadmDir}}/config.yaml

# CIS benchmark purposes - restrict permissions on PKI files
chmod 600 /etc/kubernetes/pki/*.{crt,key} /etc/kubernetes/pki/etcd/*.{crt,key}

# Restrict permissions on admin kubeconfig files for security
chmod 600 /etc/kubernetes/admin.conf /etc/kubernetes/super-admin.conf 2>/dev/null || true

# Force admin-cert auth for operations requiring elevated privileges
export BB_KUBE_AUTH_TYPE="admin-cert"
export BB_KUBE_APISERVER_URL=""
bb-curl-helper-extract-admin-certs

# This phase add 'node.kubernetes.io/exclude-from-external-load-balancers' label to node
# with this label we cannot use target load balancers to control-plane nodes, so we manually remove them
if ! bb-curl-helper-patch-node-metadata "$(hostname)" "labels" "node.kubernetes.io/exclude-from-external-load-balancers-"; then
  echo "Cannot remove node.kubernetes.io/exclude-from-external-load-balancers label from node" 1>&2
  exit 1
fi

# Upload pki for deckhouse
bb-curl-kube "/api/v1/namespaces/kube-system/secrets/d8-pki" -X DELETE || true

# Build secret JSON with base64-encoded PKI files
pki_data="{}"
for kv in \
  "ca.crt=/etc/kubernetes/pki/ca.crt" \
  "ca.key=/etc/kubernetes/pki/ca.key" \
  "sa.pub=/etc/kubernetes/pki/sa.pub" \
  "sa.key=/etc/kubernetes/pki/sa.key" \
  "front-proxy-ca.crt=/etc/kubernetes/pki/front-proxy-ca.crt" \
  "front-proxy-ca.key=/etc/kubernetes/pki/front-proxy-ca.key" \
  "etcd-ca.crt=/etc/kubernetes/pki/etcd/ca.crt" \
  "etcd-ca.key=/etc/kubernetes/pki/etcd/ca.key"; do
  key="${kv%%=*}"
  filepath="${kv#*=}"
  encoded=$(base64 -w0 < "$filepath")
  pki_data=$(jq --arg k "$key" --arg v "$encoded" '.[$k] = $v' <<< "$pki_data")
done

bb-curl-kube "/api/v1/namespaces/kube-system/secrets" \
  -X POST \
  -H "Content-Type: application/json" \
  --data "$(jq -nc --argjson data "$pki_data" \
    '{"apiVersion":"v1","kind":"Secret","metadata":{"name":"d8-pki","namespace":"kube-system"},"type":"Opaque","data":$data}')"

# Setup kubectl for root user during bootstrap.
# The control-plane-manager manages this symlink; when the user-authz module is enabled, see controlPlaneManager.rootKubeconfigSymlink.
if [ ! -f /root/.kube/config ]; then
  mkdir -p /root/.kube
  ln -s /etc/kubernetes/admin.conf /root/.kube/config
fi

