# Patches

## 001-request-ip.patch

Add the oportunity to request specific IP-address using annotation:

    cni.cilium.io/ipAddress: 10.10.10.10

Upstream <https://github.com/cilium/cilium/pull/24098>

## 002-stable-mac.patch

Use predicted MAC-address generation mechanism to make live-migration working.

Upstream <https://github.com/cilium/cilium/pull/24100>

## 002-mtu.patch

Set correct MTU value for veth interfaces

Upstream issue <https://github.com/cilium/cilium/issues/23711>
