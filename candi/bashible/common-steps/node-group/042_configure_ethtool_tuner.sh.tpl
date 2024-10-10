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

bb-event-on 'ethtool-tuner-service-changed' '_enable_ethtool_tuner_service'
function _enable_ethtool_tuner_service() {
  systemctl daemon-reload
  systemctl enable ethtool-tuner.timer
}

bb-sync-file /opt/deckhouse/bin/ethtool-tuner - << "EOF"
#!/bin/bash

if ! [ -x "$(command -v /opt/deckhouse/bin/ethtool)" ]; then
  echo "ethtool is not founded"
  exit 0
fi
ifaces=( $(ip -json a | /opt/deckhouse/bin/jq -r '.[].ifname') )
for iface in "${ifaces[@]}"; do
  if [[ "$iface" == "lo" ]]; then
    continue
  fi
  driver=$(/opt/deckhouse/bin/ethtool -i $iface | grep driver | cut -d':' -f2 | tr -d '[:space:]')
  if [[ "$driver" == "vmxnet3" ]]; then
    tnlsegmentation="$(/opt/deckhouse/bin/ethtool -k "$iface" | grep tx-udp_tnl-segmentation | cut -d':' -f2 | tr -d '[:space:]')"
    if [[ "$tnlsegmentation" == "on" ]]; then
      /opt/deckhouse/bin/ethtool -K $iface tx-udp_tnl-segmentation off
      echo "disabling tx-udp_tnl-segmentation for interface: ${iface}"
    fi

    csumsegmentation="$(/opt/deckhouse/bin/ethtool -k "$iface" | grep tx-udp_tnl-csum-segmentation | cut -d':' -f2 | tr -d '[:space:]')"
    if [[ "$csumsegmentation" == "on" ]]; then
      /opt/deckhouse/bin/ethtool -K $iface tx-udp_tnl-csum-segmentation off
      echo "disabling tx-udp_tnl-csum-segmentation for interface: ${iface}"
    fi
  fi
done
EOF
chmod +x /opt/deckhouse/bin/ethtool-tuner

# Generate ethtool tuner unit
bb-sync-file /etc/systemd/system/ethtool-tuner.timer - ethtool-tuner-service-changed << EOF
[Unit]
Description=Ethtool Tuner timer

[Timer]
OnBootSec=1min
OnUnitActiveSec=10min

[Install]
WantedBy=multi-user.target
EOF

bb-sync-file /etc/systemd/system/ethtool-tuner.service - ethtool-tuner-service-changed << EOF
[Unit]
Description=Ethtool Tuner

[Service]
EnvironmentFile=/etc/environment
ExecStart=/opt/deckhouse/bin/ethtool-tuner
EOF

systemctl stop ethtool-tuner.timer
