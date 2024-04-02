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

current_kubeadm_checksum="$(find /var/lib/bashible/kubeadm -type f -name '*.yaml' | sort | xargs cat | md5sum - | awk '{print $1}')"

if [ -f /.kubeadm.checksum ]; then
  previous_kubeadm_checksum="$(</.kubeadm.checksum)"
  if [[ "$current_kubeadm_checksum" == "$previous_kubeadm_checksum" ]]; then
    exit 0
  fi
else
  if [ -f /etc/kubernetes/admin.conf ]; then
    >&2 echo "ERROR: Trying to re-bootstrap cluster which was bootstrapped more than 2h ago. To force re-bootstrap: touch /.kubeadm.checksum"
    exit 1
  fi
fi

if [ -f /etc/kubernetes/admin.conf ]; then
  if ! bb-kubectl --kubeconfig /etc/kubernetes/admin.conf version > /dev/null; then
    for i in $(seq 60 -1 1); do
      echo  "WARNING: Cluster will be re-bootstrapped, all data will be lost, in $i sec"
      sleep 1
    done

  elif bb-kubectl --kubeconfig /etc/kubernetes/admin.conf get nodes -o name | grep -q -v "^node/${D8_NODE_HOSTNAME}$"; then
    >&2 echo "ERROR: Trying to re-bootstrap cluster which has more than one node."
    exit 1
  fi

  kubeadm reset -f
fi

echo "$current_kubeadm_checksum" > /.kubeadm.checksum
