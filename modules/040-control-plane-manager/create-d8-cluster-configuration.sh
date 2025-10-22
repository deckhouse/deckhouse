#!/bin/bash

# Copyright 2022 Flant JSC
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

set -ex

provider=""
clusterType="Cloud"
p=$(deckhouse-controller module list | grep cloud-provider | cut -d- -f3 | sed 's/ *$//g')

case "$p" in
  'aws')
    provider="AWS"
    ;;
  'gcp')
    provider="GCP"
    ;;
  'openstack')
    provider="OpenStack"
    ;;
  'yandex')
    provider="Yandex"
    ;;
  'vsphere')
    provider="Vsphere"
    ;;
  * )
    clusterType="Static"
    ;;
esac

#check if kops
if kubectl -n kube-system get pods -l k8s-app=kube-controller-manager -o yaml | grep 'cloud-provider' > /dev/null 2> /dev/null ; then
  echo "Error: script must not be executed on kops clusters"
  exit 1
fi

#check if standalone etcd
if [[ "x$(kubectl -n kube-system get pods -l component=etcd,tier=control-plane -o name | wc -l)" != "x1" ]] ; then
  echo "Error: script must be executed only on standalone etcd"
  exit 1
fi

#check if d8-pki exists
if ! kubectl -n kube-system get secret d8-pki > /dev/null 2> /dev/null ; then
  echo "Error: d8-pki secret in namespace kube-system doesn't exists"
  exit 1
fi

masterInternalIP=$(kubectl get nodes -l node-role.kubernetes.io/control-plane='' -o json | jq '.items[0].status.addresses[] | select(.type == "InternalIP") | .address' -r)
etcdMemberList=$(kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1 | cut -d/ -f2) -- etcdctl --ca-file /etc/kubernetes/pki/etcd/ca.crt --cert-file /etc/kubernetes/pki/etcd/ca.crt --key-file /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list)

#check etcd clientURLs
if ! echo "$etcdMemberList" | grep -o "clientURLs=[^ ]*" | grep "$masterInternalIP"; then
  echo "Error: etcd must listen for clients on master internal ip"
  exit 1
fi

#check etcd peerURLs
if ! echo "$etcdMemberList" | grep -o "peerURLs=[^ ]*" | grep "$masterInternalIP"; then
  echo "Error: etcd must listen for peers on master internal ip"
  exit 1
fi

apiserverAdvertiseAddress=$(kubectl -n default get ep kubernetes -o json | jq '.subsets[0].addresses[0].ip' -r)
#check apiserver on correct ip and controlPlaneManager configuration
if [[ "x$apiserverAdvertiseAddress" != "x$masterInternalIP" ]]; then
  if [[ "x$(deckhouse-controller module values control-plane-manager | yq r - controlPlaneManager.apiserver.bindToWildcard)" != "xtrue" ]]; then
    echo "Error: apiserver advertise address differs from master internal ip, so you must set controlPlaneManager.apiserver.bindToWildcard to true"
    exit 1
  fi

  if ! deckhouse-controller module values control-plane-manager | yq r - controlPlaneManager.apiserver.certSANs -j | jq -e ". | index(\"$apiserverAdvertiseAddress\")"; then
    echo "Error: apiserver advertise address must be added to controlPlaneManager.apiserver.certSANs"
    exit 1
  fi
fi

#check if kubeadm config exists
if ! kubectl -n kube-system get cm kubeadm-config > /dev/null 2> /dev/null ; then
  echo "Error: kubeadm-config cm in namespace kube-system doesn't exists"
  exit 1
fi

cluster_configuration=''
if [[ $clusterType == "Static" ]]; then
  cluster_configuration=$(cat <<END
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: $clusterType
podSubnetCIDR: $(deckhouse-controller global values | yq r - global.discovery.podSubnet)
podSubnetNodeCIDRPrefix: $(kubectl -n kube-system get cm kubeadm-config -o yaml | yq r - data.ClusterConfiguration | yq r -j - controllerManager.extraArgs | jq '."node-cidr-mask-size" //= "24" | ."node-cidr-mask-size"')
serviceSubnetCIDR: $(deckhouse-controller global values | yq r - global.discovery.serviceSubnet)
kubernetesVersion: "$(deckhouse-controller global values | yq r - global.discovery.kubernetesVersion | cut -c 1-4)"
clusterDomain: $(deckhouse-controller global values | yq r - global.discovery.clusterDomain)
END
)
elif [[ $clusterType == "Cloud" ]]; then
  cluster_configuration=$(cat <<END
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: $clusterType
cloud:
  provider: $provider
  prefix: $(deckhouse-controller module values node-manager | yq r - nodeManager.instancePrefix)
podSubnetCIDR: $(deckhouse-controller global values | yq r - global.discovery.podSubnet)
podSubnetNodeCIDRPrefix: $(kubectl -n kube-system get cm kubeadm-config -o yaml | yq r - data.ClusterConfiguration | yq r -j - controllerManager.extraArgs | jq '."node-cidr-mask-size" //= "24" | ."node-cidr-mask-size"')
serviceSubnetCIDR: $(deckhouse-controller global values | yq r - global.discovery.serviceSubnet)
kubernetesVersion: "$(deckhouse-controller global values | yq r - global.discovery.kubernetesVersion | cut -c 1-4)"
clusterDomain: $(deckhouse-controller global values | yq r - global.discovery.clusterDomain)
END
)
fi

cat <<END
##### check cluster_configuration.yaml#######
$cluster_configuration
#############################################
END

if [[ $1 == "dry-run" ]]; then
  exit 0
fi

#check if d8-cluster-configuration already exists
if kubectl -n kube-system get secret d8-cluster-configuration > /dev/null 2> /dev/null ; then
  echo "Error: d8-cluster-configuration secret in namespace kube-system already exists"
  exit 1
fi

kubectl create -f - <<END
apiVersion: v1
data:
  cluster-configuration.yaml: $(echo "$cluster_configuration" | base64 -w 0)
kind: Secret
metadata:
  labels:
    validation.deckhouse.io/selector: d8-cluster-configuration
  name: d8-cluster-configuration
  namespace: kube-system
type: Opaque
END
