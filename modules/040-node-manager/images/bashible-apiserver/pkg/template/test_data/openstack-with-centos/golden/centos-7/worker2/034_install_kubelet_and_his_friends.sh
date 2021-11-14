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
bb-rp-remove kubeadm
bb-rp-install "kubernetes-cni:4f94e259a70e9b83fd085b647121baa0e11d13cc5eded69a7a626764-1638993151548" "kubelet:063265ad04329d34f3ed5810f2c849cc22caeff744c84a037b094e1a-1638990871366" "kubectl:456fd57953945f4c97b349cded51c8933e528211ec81a3fc806caf64-1638990835470"

if [[ "$FIRST_BASHIBLE_RUN" == "yes" && ! -f /etc/systemd/system/kubelet.service.d/10-deckhouse.conf ]] && systemctl is-active -q kubelet; then
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
mkdir -p /var/lib/kubelet
