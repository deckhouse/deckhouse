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

# Sub-step timing — emit `[bashible-timing] step=071_install_control_plane.sh
# section=<name> dur=<sec>` so dhctl-side parsing can attribute the (often >200s)
# total of this step to one of: image-pull waits, kubelet Node registration,
# RBAC/label/taint setup, or PKI upload.
__sec_start=$(date +%s.%N)
__sec() {
  local now dur
  now=$(date +%s.%N)
  dur=$(awk -v s="$__sec_start" -v e="$now" 'BEGIN{printf "%.3f", e-s}')
  echo "[bashible-timing] step=071_install_control_plane.sh section=$1 dur=${dur}s"
  __sec_start=$now
}

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
__sec wait_api_proxy

cp -r {{ $manifestsDir}}/pki /etc/kubernetes/
cp {{ $kubeconfigDir }}/{admin.conf,controller-manager.conf,scheduler.conf,super-admin.conf} /etc/kubernetes/

# etcd runs as UID/GID 52 (etcd user) with capabilities: drop: ALL.
# Without CAP_DAC_OVERRIDE the process cannot read root-owned 600 files.
# Set ownership and modes before the etcd container starts:
#   directory  root:etcd 750  — etcd (GID 52) gets r-x; kube-apiserver (UID 0 = owner) gets rwx
#   *.key      etcd:etcd 600  — private keys readable only by etcd
#   *.crt      etcd:etcd 644  — public certs; kube-apiserver reads ca.crt via "other" r bit
chown root:etcd /etc/kubernetes/pki/etcd
chmod 750 /etc/kubernetes/pki/etcd
chown etcd:etcd /etc/kubernetes/pki/etcd/*.{crt,key}
chmod 600 /etc/kubernetes/pki/etcd/*.key
chmod 644 /etc/kubernetes/pki/etcd/*.crt

cp {{ $manifestsDir}}/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
check_container_running "etcd" || exit 1
__sec wait_etcd

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
__sec wait_cp_trio

node_name="$(bb-d8-node-name)"

export BB_KUBE_AUTH_TYPE="super-admin-cert"
export BB_KUBE_APISERVER_URL=""
bb-curl-helper-extract-super-admin-certs

# admin.conf authenticates as user "kubernetes-admin" in group
# "kubeadm:cluster-admins" — but that group has NO permissions until the
# kubeadm:cluster-admins ClusterRoleBinding exists. If the Node-wait loop
# below used admin.conf before creating the binding, every Node GET would
# 403 and the loop would burn its full 200s timeout instead of breaking on
# the first successful poll (~220s measured on every fresh bootstrap).
# So: bind via super-admin certs first (they bypass RBAC), then wait.
# Idempotent via get-or-create guard (step is bashible-retried on failure).
if ! bb-curl-kube "/apis/rbac.authorization.k8s.io/v1/clusterrolebindings/kubeadm:cluster-admins" >/dev/null 2>&1; then
  bb-curl-kube "/apis/rbac.authorization.k8s.io/v1/clusterrolebindings" \
    -X POST \
    -H "Content-Type: application/json" \
    --data @- <<'EOF'
{
  "apiVersion": "rbac.authorization.k8s.io/v1",
  "kind": "ClusterRoleBinding",
  "metadata": { "name": "kubeadm:cluster-admins" },
  "roleRef": {
    "apiGroup": "rbac.authorization.k8s.io",
    "kind": "ClusterRole",
    "name": "cluster-admin"
  },
  "subjects": [
    {
      "apiGroup": "rbac.authorization.k8s.io",
      "kind": "Group",
      "name": "kubeadm:cluster-admins"
    }
  ]
}
EOF
fi

export BB_KUBE_AUTH_TYPE="admin-cert"
export BB_KUBE_APISERVER_URL=""
bb-curl-helper-extract-admin-certs

# kubelet registers the Node object slightly after the control-plane containers
# report running, so the label/taint below can race ahead of it. Wait for the
# Node to appear (≤200s) before touching it.
for _ in $(seq 1 100); do
  if bb-curl-kube "/api/v1/nodes/${node_name}" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done
__sec wait_node_register

node_taints_patch="$(
  bb-curl-kube "/api/v1/nodes/${node_name}" | jq -c '
    (.spec.taints // [])
    | map(select(.key != "node-role.kubernetes.io/control-plane"))
    + [{"key":"node-role.kubernetes.io/control-plane","effect":"NoSchedule"}]
  '
)"

bb-curl-helper-patch-node-metadata "$node_name" "labels" "node-role.kubernetes.io/control-plane="
bb-curl-kube "/api/v1/nodes/${node_name}" \
  -X PATCH \
  -H "Content-Type: application/strategic-merge-patch+json" \
  --data "$(jq -nc --argjson taints "$node_taints_patch" '{"spec":{"taints":$taints}}')"
__sec rbac_label_taint

# CIS benchmark purposes
chmod 600 /etc/kubernetes/pki/*.{crt,key} /etc/kubernetes/pki/etcd/*.{crt,key}
# etcd and kube-apiserver both run with capabilities: drop: ALL (no CAP_DAC_OVERRIDE).
# etcd owns the cert files (etcd:etcd); kube-apiserver (UID 0) needs to read ca.crt
# on any restart via the "other" r bit. Keys stay 600 (private, only etcd reads them).
chmod 644 /etc/kubernetes/pki/etcd/*.crt

# Restrict permissions on admin kubeconfig files for security
chmod 600 /etc/kubernetes/admin.conf /etc/kubernetes/super-admin.conf 2>/dev/null || true

# Upload pki for deckhouse
declare -A pki_secret_files=(
  ["ca.crt"]="/etc/kubernetes/pki/ca.crt"
  ["ca.key"]="/etc/kubernetes/pki/ca.key"
  ["sa.pub"]="/etc/kubernetes/pki/sa.pub"
  ["sa.key"]="/etc/kubernetes/pki/sa.key"
  ["front-proxy-ca.crt"]="/etc/kubernetes/pki/front-proxy-ca.crt"
  ["front-proxy-ca.key"]="/etc/kubernetes/pki/front-proxy-ca.key"
  ["etcd-ca.crt"]="/etc/kubernetes/pki/etcd/ca.crt"
  ["etcd-ca.key"]="/etc/kubernetes/pki/etcd/ca.key"
)

declare -A const_signatures_files=(
  ["signature-private"]="/etc/kubernetes/pki/signature-private.jwk"
  ["signature-public"]="/etc/kubernetes/pki/signature-public.jwks"
)

declare -A have_signatures_files

for sig_key in "${!const_signatures_files[@]}"; do
  sig_key_path="${const_signatures_files[$sig_key]}"
  if [ -f "$sig_key_path" ]; then
    have_signatures_files["${sig_key}"]="$sig_key_path"
  fi
done

if [[ "${#have_signatures_files[@]}" != "0" ]]; then
  if [[ "${#have_signatures_files[@]}" != "${#const_signatures_files[@]}" ]]; then
    bb-log-error "Internal error: not enough signatures files! Have keys ${have_signatures_files[*]}"
    exit 1
  fi

  bb-log-info "Have signatures files. Add to create pki secret create args"

  for have_sig_key in "${!have_signatures_files[@]}"; do
    pki_secret_files["${have_sig_key}"]="${have_signatures_files[$have_sig_key]}"
  done
fi

bb-curl-kube "/api/v1/namespaces/kube-system/secrets/d8-pki" -X DELETE || true

# Build secret JSON with base64-encoded PKI files
pki_data="{}"
for pki_key in "${!pki_secret_files[@]}"; do
  pki_key_path="${pki_secret_files[$pki_key]}"
  if [ ! -f "$pki_key_path" ]; then
    bb-log-error "Internal error: pki file $pki_key_path not file or not found"
    exit 1
  fi
  encoded=$(base64 -w0 < "$pki_key_path" )
  pki_data=$(jq --arg k "$pki_key" --arg v "$encoded" '.[$k] = $v' <<< "$pki_data")
done

bb-curl-kube "/api/v1/namespaces/kube-system/secrets" \
  -X POST \
  -H "Content-Type: application/json" \
  --data "$(jq -nc --argjson data "$pki_data" \
    '{"apiVersion":"v1","kind":"Secret","metadata":{"name":"d8-pki","namespace":"kube-system"},"type":"Opaque","data":$data}')"
__sec pki_upload

# Setup kubectl for root user during bootstrap.
# The control-plane-manager manages this symlink; when the user-authz module is enabled, see controlPlaneManager.rootKubeconfigSymlink.
if [ ! -f /root/.kube/config ]; then
  mkdir -p /root/.kube
  ln -s /etc/kubernetes/admin.conf /root/.kube/config
fi
# Allow kube-apiserver to proxy kubelet requests (kubectl logs/exec/port-forward) during bootstrap.
# The same CRB is applied with heritage/module labels by module 040-control-plane-manager; nelm adopts it.
# Idempotent via get-or-create guard (step is bashible-retried on failure).
if ! bb-curl-kube "/apis/rbac.authorization.k8s.io/v1/clusterrolebindings/d8:control-plane-manager:apiserver-kubelet-client" >/dev/null 2>&1; then
  bb-curl-kube "/apis/rbac.authorization.k8s.io/v1/clusterrolebindings" \
    -X POST \
    -H "Content-Type: application/json" \
    --data @- <<'EOF'
{
  "apiVersion": "rbac.authorization.k8s.io/v1",
  "kind": "ClusterRoleBinding",
  "metadata": { "name": "d8:control-plane-manager:apiserver-kubelet-client" },
  "roleRef": {
    "apiGroup": "rbac.authorization.k8s.io",
    "kind": "ClusterRole",
    "name": "system:kubelet-api-admin"
  },
  "subjects": [
    {
      "apiGroup": "rbac.authorization.k8s.io",
      "kind": "User",
      "name": "kube-apiserver-kubelet-client"
    }
  ]
}
EOF
fi
