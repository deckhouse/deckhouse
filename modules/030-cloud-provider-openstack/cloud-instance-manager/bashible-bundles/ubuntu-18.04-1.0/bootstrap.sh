#!/bin/bash

# {{ .instanceGroup.name }}
# {{ .zoneName }}
# {{ .Values.cloudInstanceManager.internal.cloudProvider.openstack.publicNetworkName }}
# {{ .Values.cloudInstanceManager.internal.cloudProvider.openstack.internalNetworkName }}

internal_iface_name=$(ip -brief link | grep DOWN | grep -v docker0 | awk '{print $1}')
if [[ -n "$internal_iface_name" ]]; then
  ip link set dev "$internal_iface_name" name internal
elif ! ip -brief link | grep -q internal; then
  2>&1 echo "FATAL: \"internal\" interface could not be detected"
  exit 1
fi

internal_iface_mac=$(ip -brief link show dev internal | awk '{print $3}')
if [[ -n "$internal_iface_mac" ]]; then
  cat << EOF > /etc/netplan/20-internal.yaml
network:
    version: 2
    ethernets:
        internal:
            dhcp4: true
            set-name: internal
            match:
                macaddress: $internal_iface_mac
EOF
  netplan apply
else
  2>&1 echo "FATAL: \"internal\" interface's MAC-address could not be detected"
  exit 1
fi
