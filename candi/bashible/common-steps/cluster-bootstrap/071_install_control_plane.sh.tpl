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

{{ $manifestsDir := "/var/lib/bashible/control-plane" }}
{{ $kubeconfigDir := "/var/lib/bashible/control-plane/kubeconfig" }}

# Poll crictl every 2s instead of every 10s — kubelet starts static pods within seconds,
# so 10s granularity wasted 8-10s per container in practice. Total timeout preserved at 200s.
check_container_running() {
  local container_name=$1
  local max_retries=100
  local sleep_interval=2
  local count=0

  while [[ $count -lt $max_retries ]]; do
    if crictl ps -o json | jq -e --arg name "$container_name" '.containers[] | select(.metadata.name == $name and .state == "CONTAINER_RUNNING")' > /dev/null; then
      echo "$container_name is running"
      return 0
    fi
    count=$((count + 1))

    if [[ $count -ge $max_retries ]]; then
      echo "$container_name not running after $((sleep_interval * max_retries))s" >&2
      return 1
    fi

    sleep $sleep_interval
  done
}

check_container_running "kubernetes-api-proxy" || exit 1

cp -r {{ $manifestsDir}}/pki /etc/kubernetes/
cp {{ $kubeconfigDir }}/{admin.conf,controller-manager.conf,scheduler.conf,super-admin.conf} /etc/kubernetes/
cp {{ $manifestsDir}}/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
check_container_running "etcd" || exit 1

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

# Copy all three manifests at once — kubelet starts the static pods concurrently,
# so we can wait for them concurrently too. Without this each check_container_running
# blocked for the full kubelet/container start latency serially (~3-5s each wasted).
cp {{ $manifestsDir}}/kube-apiserver.yaml /etc/kubernetes/manifests/kube-apiserver.yaml
cp {{ $manifestsDir}}/kube-scheduler.yaml /etc/kubernetes/manifests/kube-scheduler.yaml
cp {{ $manifestsDir}}/kube-controller-manager.yaml /etc/kubernetes/manifests/kube-controller-manager.yaml

check_container_running "kube-apiserver" &
pid_api=$!
check_container_running "kube-controller-manager" &
pid_cm=$!
check_container_running "kube-scheduler" &
pid_sched=$!

cp_failed=0
wait $pid_api || cp_failed=1
wait $pid_cm || cp_failed=1
wait $pid_sched || cp_failed=1
if [[ $cp_failed -ne 0 ]]; then
  echo "one or more control-plane containers failed to start" >&2
  exit 1
fi

kubectl --kubeconfig=/etc/kubernetes/super-admin.conf create clusterrolebinding kubeadm:cluster-admins --clusterrole=cluster-admin --group=kubeadm:cluster-admins
kubectl --kubeconfig=/etc/kubernetes/admin.conf label node "$(bb-d8-node-name)" node-role.kubernetes.io/control-plane=""
kubectl --kubeconfig=/etc/kubernetes/admin.conf taint node "$(bb-d8-node-name)" node-role.kubernetes.io/control-plane:NoSchedule

# CIS benchmark purposes
chmod 600 /etc/kubernetes/pki/*.{crt,key} /etc/kubernetes/pki/etcd/*.{crt,key}

# Restrict permissions on admin kubeconfig files for security
chmod 600 /etc/kubernetes/admin.conf /etc/kubernetes/super-admin.conf 2>/dev/null || true

# Force admin-cert auth for operations requiring elevated privileges
export BB_KUBE_AUTH_TYPE="admin-cert"
export BB_KUBE_APISERVER_URL=""
bb-curl-helper-extract-admin-certs

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

