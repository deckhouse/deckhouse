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
bb-rp-install "kubernetes-cni:294c29c0c84131aaa167294bbecd75c62bd2ef6079c3e7f8e6911c74-1638477359972" "kubelet:a0ebe5c4b0da1252340d80da31979fd9c0bf6b7ed1515d62778dabcb-1638797344737" "kubectl:502a5656f10f47da33311595ec92c692909f730a5fe0f9cf5091a1b0-1638797320153"

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
