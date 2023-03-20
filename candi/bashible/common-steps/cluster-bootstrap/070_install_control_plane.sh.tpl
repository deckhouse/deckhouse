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

mkdir -p /etc/kubernetes/deckhouse/kubeadm/patches/
cp /var/lib/bashible/kubeadm/patches/* /etc/kubernetes/deckhouse/kubeadm/patches/
kubeadm init phase certs all --config /var/lib/bashible/kubeadm/config.yaml
kubeadm init phase kubeconfig all --config /var/lib/bashible/kubeadm/config.yaml
kubeadm init phase etcd local --config /var/lib/bashible/kubeadm/config.yaml
kubeadm init phase control-plane all --config /var/lib/bashible/kubeadm/config.yaml
kubeadm init phase mark-control-plane --config /var/lib/bashible/kubeadm/config.yaml
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

{{- if semverCompare "<1.24" .kubernetesVersion }}
# add node-role.kubernetes.io/control-plane taint
# kubeadm < 1.24 taint node only with add node-role.kubernetes.io/master taint
if ! bb-kubectl --kubeconfig=/etc/kubernetes/admin.conf taint node "$(hostname)" node-role.kubernetes.io/control-plane=:NoSchedule; then
  echo "Cannot add 'node-role.kubernetes.io/control-plane' taint to node" 1>&2
  exit 1
fi
{{- end }}

{{- if semverCompare "<1.25" .kubernetesVersion }}
# remove node-role.kubernetes.io/master taint because we start use node-role.kubernetes.io/control-plane
# and node-role.kubernetes.io/master keeps only first master node
if ! bb-kubectl --kubeconfig=/etc/kubernetes/admin.conf taint node "$(hostname)" node-role.kubernetes.io/master-; then
  echo "Cannot remove 'node-role.kubernetes.io/master' taint from node" 1>&2
  exit 1
fi
{{- end }}
