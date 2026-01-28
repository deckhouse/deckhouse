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

{{ $kubernetesVersion := .kubernetesVersion | toString }}
{{ $kubeadmDir := ternary "/var/lib/bashible/kubeadm/v1beta4" "/var/lib/bashible/kubeadm/v1beta3" (semverCompare ">=1.31" $kubernetesVersion) }}

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

check_container_running "kubernetes-api-proxy-reloader"
check_container_running "kubernetes-api-proxy"
mkdir -p /etc/kubernetes/deckhouse/kubeadm/patches/
cp {{ $kubeadmDir}}/patches/* /etc/kubernetes/deckhouse/kubeadm/patches/
kubeadm init phase certs all --config {{ $kubeadmDir}}/config.yaml
kubeadm init phase kubeconfig all --config {{ $kubeadmDir}}/config.yaml
kubeadm init phase etcd local --config {{ $kubeadmDir}}/config.yaml
check_container_running "etcd"
kubeadm init phase control-plane all --config {{ $kubeadmDir}}/config.yaml
check_container_running "kube-apiserver"
check_container_running "healthcheck"
check_container_running "kube-controller-manager"
check_container_running "kube-scheduler"
kubeadm init phase mark-control-plane --config {{ $kubeadmDir}}/config.yaml

# CIS becnhmark purposes
chmod 600 /etc/kubernetes/pki/*.{crt,key} /etc/kubernetes/pki/etcd/*.{crt,key}

# This phase add 'node.kubernetes.io/exclude-from-external-load-balancers' label to node
# with this label we cannot use target load balancers to control-plane nodes, so we manually remove them
if ! bb-kubectl --kubeconfig=/etc/kubernetes/admin.conf label node "$(hostname)" node.kubernetes.io/exclude-from-external-load-balancers-; then
  echo "Cannot remove node.kubernetes.io/exclude-from-external-load-balancers label from node" 1>&2
  exit 1
fi

# Upload pki for deckhouse
bb-kubectl --kubeconfig=/etc/kubernetes/admin.conf -n kube-system delete secret d8-pki || true
bb-kubectl --kubeconfig=/etc/kubernetes/admin.conf -n kube-system create secret generic d8-pki \
  --from-file=ca.crt=/etc/kubernetes/pki/ca.crt \
  --from-file=ca.key=/etc/kubernetes/pki/ca.key \
  --from-file=sa.pub=/etc/kubernetes/pki/sa.pub \
  --from-file=sa.key=/etc/kubernetes/pki/sa.key \
  --from-file=front-proxy-ca.crt=/etc/kubernetes/pki/front-proxy-ca.crt \
  --from-file=front-proxy-ca.key=/etc/kubernetes/pki/front-proxy-ca.key \
  --from-file=etcd-ca.crt=/etc/kubernetes/pki/etcd/ca.crt \
  --from-file=etcd-ca.key=/etc/kubernetes/pki/etcd/ca.key

# Setup kubectl for root user
if [ ! -f /root/.kube/config ]; then
  mkdir -p /root/.kube
  ln -s /etc/kubernetes/admin.conf /root/.kube/config
fi

