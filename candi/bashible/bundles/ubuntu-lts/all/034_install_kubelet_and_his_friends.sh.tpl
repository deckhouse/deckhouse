# Copyright 2021 Flant CJSC
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

kubernetes_version="{{ printf "%s.%s-00" (.kubernetesVersion | toString ) (index .k8s .kubernetesVersion "patch" | toString) }}"
kubernetes_cni_version="{{ printf "%s-00" (index .k8s .kubernetesVersion "cni_version" | toString) }}"

if dpkg -S kubelet >/dev/null 2>&1; then
  kubernetes_current_version="$(dpkg -s kubelet | awk '/Version/{print $2}')"
  if grep "^1.15" <<< "$kubernetes_version" >/dev/null && grep "^1.16" <<< "$kubernetes_current_version" >/dev/null; then
    bb-deckhouse-get-disruptive-update-approval
  fi
  if grep "^1.16" <<< "$kubernetes_version" >/dev/null && grep "^1.15" <<< "$kubernetes_current_version" >/dev/null; then
    bb-deckhouse-get-disruptive-update-approval
  fi
fi

bb-apt-remove kubeadm
bb-apt-install "kubelet=${kubernetes_version}" "kubectl=${kubernetes_version}" "kubernetes-cni=${kubernetes_cni_version}"

if [[ "$FIRST_BASHIBLE_RUN" == "yes" && ! -f /etc/systemd/system/kubelet.service.d/10-deckhouse.conf ]]; then
  # stop kubelet immediately after the first install to prevent joining to the cluster with wrong configurations
  systemctl stop kubelet
fi

if kubelet_pid="$(pidof kubelet)"; then
  kubelet_start_date="$(ps -o lstart= -q "$kubelet_pid")"
  kubelet_start_unixtime="$(date --date="$kubelet_start_date" +%s)"
  kubelet_bin_change_unixtime="$(stat -c %Z /usr/bin/kubelet)"

  if [ "$kubelet_bin_change_unixtime" -gt "$kubelet_start_unixtime" ]; then
    bb-flag-set kubelet-need-restart
  fi
fi

mkdir -p /etc/kubernetes/manifests
mkdir -p /etc/systemd/system/kubelet.service.d
mkdir -p /etc/kubernetes/pki
