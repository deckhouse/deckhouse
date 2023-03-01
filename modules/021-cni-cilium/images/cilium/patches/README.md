# Patches

## 001-customer-annotations.patch

Add the oportunity to request specific MAC- and IP-address using annotations:

    cni.cilium.io/ipAddress: 10.10.10.10
    cni.cilium.io/macAddress: f6:e1:74:94:b8:1d

Upstream <https://github.com/cilium/cilium/pull/19789>

## 002-mtu.patch

Set correct MTU value for veth interfaces

Upstream issue <https://github.com/cilium/cilium/issues/23711>
