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

# Files matching this glob are concatenated (sorted) and hashed.
current_controlplane_checksum="$(
  { find {{ $manifestsDir }} -maxdepth 1 -type f -name '*.yaml' 2>/dev/null | sort | xargs -r cat;
    find {{ $manifestsDir }}/kubeconfig -maxdepth 1 -type f -name '*.conf' 2>/dev/null | sort | xargs -r cat;
  } | md5sum - | awk '{print $1}'
)"

if [ -f /.controlplane.checksum ]; then
  previous_controlplane_checksum="$(</.controlplane.checksum)"
  if [[ "$current_controlplane_checksum" == "$previous_controlplane_checksum" ]]; then
    exit 0
  fi
else
  if [ -f /etc/kubernetes/admin.conf ]; then
    >&2 echo "ERROR: Trying to re-bootstrap cluster which was bootstrapped more than 2h ago. To force re-bootstrap: touch /.controlplane.checksum"
    exit 1
  fi
fi

if [ -f /etc/kubernetes/admin.conf ]; then
  export BB_KUBE_AUTH_TYPE="admin-cert"
  export BB_KUBE_APISERVER_URL=""
  bb-curl-helper-extract-admin-certs

  if ! bb-curl-kube "/version" > /dev/null 2>&1; then
    for i in $(seq 60 -1 1); do
      echo  "WARNING: Cluster will be re-bootstrapped, all data will be lost, in $i sec"
      sleep 1
    done

  elif bb-curl-kube "/api/v1/nodes" | jq -r '.items[].metadata.name' | grep -q -v "^$(bb-d8-node-name)$"; then
    >&2 echo "ERROR: Trying to re-bootstrap cluster which has more than one node."
    exit 1
  fi

  >&2 echo "WARNING: Resetting local control-plane state (manifests, pki, kubeconfigs) for re-bootstrap."
  rm -f /etc/kubernetes/manifests/etcd.yaml \
        /etc/kubernetes/manifests/kube-apiserver.yaml \
        /etc/kubernetes/manifests/kube-controller-manager.yaml \
        /etc/kubernetes/manifests/kube-scheduler.yaml
  rm -rf /etc/kubernetes/pki
  rm -f /etc/kubernetes/admin.conf \
        /etc/kubernetes/super-admin.conf \
        /etc/kubernetes/controller-manager.conf \
        /etc/kubernetes/scheduler.conf \
        /etc/kubernetes/kubelet.conf
  rm -rf /var/lib/etcd
fi

echo "$current_controlplane_checksum" > /.controlplane.checksum
