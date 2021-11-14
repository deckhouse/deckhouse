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

kubernetes_version="1.21.7"
kubernetes_cni_version="0.8.7"

bb-rp-remove kubeadm

bb-rp-install "kubernetes-cni:${kubernetes_cni_version}-centos7" "kubelet:${kubernetes_version}-centos7" "kubectl:${kubernetes_version}-centos7"

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
