# Patches

## 001-request-ip.patch

Add the oportunity to request specific IP-address using annotation:

    cni.cilium.io/ipAddress: 10.10.10.10

Upstream <https://github.com/cilium/cilium/pull/24098>
Patch <https://patch-diff.githubusercontent.com/raw/cilium/cilium/pull/24098.patch>

## 002-stable-mac.patch

Use predefined MAC-addresses for virtualization workloads

Upstream <https://github.com/cilium/cilium/pull/24100>
Patch <https://patch-diff.githubusercontent.com/raw/cilium/cilium/pull/24100.patch>

## 003-mtu.patch

Set correct MTU value for veth interfaces

Upstream issue <https://github.com/cilium/cilium/issues/23711>
Upstream <https://github.com/cilium/cilium/pull/23712>
Patch <https://patch-diff.githubusercontent.com/raw/cilium/cilium/pull/23712.patch>
