# Copyright 2024 Flant JSC
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

# Fixing a problem with segmentation of tunnel UDP packets on interfaces with the VMware vmxnet3 driver
# The problem is observed when turning on Cilium VXLAN.
bb-log-info "disabling packet segmentation for network interfaces"
if ! [ -x "$(command -v ethtool)" ]; then
  bb-log-warning "ethtool is not founded"
  exit 0
fi
ifaces=( $(ip -json a | jq -r '.[].ifname') )
for iface in "${ifaces[@]}"; do
  if [[ "$iface" == "lo" ]]; then
    continue
  fi
  driver=$(ethtool -i $iface | grep driver | cut -d':' -f2 | tr -d '[:space:]')
  if [[ "$driver" == "vmxnet3" ]]; then
    tnlsegmentation="$(ethtool -k "$iface" | grep tx-udp_tnl-segmentation | cut -d':' -f2 | tr -d '[:space:]')"
    if [[ "$tnlsegmentation" == "on" ]]; then
      ethtool -K $iface tx-udp_tnl-segmentation off
      bb-log-info "disabling tx-udp_tnl-segmentation for interface: ${iface}"
    fi

    csumsegmentation="$(ethtool -k "$iface" | grep tx-udp_tnl-csum-segmentation | cut -d':' -f2 | tr -d '[:space:]')"
    if [[ "$csumsegmentation" == "on" ]]; then
      ethtool -K $iface tx-udp_tnl-csum-segmentation off
      bb-log-info "disabling tx-udp_tnl-csum-segmentation for interface: ${iface}"
    fi
  fi
done
