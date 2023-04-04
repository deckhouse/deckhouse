# Patches

## 001-request-ip.patch

Add the oportunity to request specific IP-address using annotation:

    cni.cilium.io/ipAddress: 10.10.10.10

Upstream <https://github.com/cilium/cilium/pull/24098>

## 002-stable-mac.patch

Use predefined MAC-addresses for virtualization workloads

Upstream <https://github.com/cilium/cilium/pull/24100>

## 003-mtu.patch

Set correct MTU value for veth interfaces

Upstream issue <https://github.com/cilium/cilium/issues/23711>

## 004-fib.patch

Perform routing lookup for custom tables

Upstream <https://github.com/cilium/cilium/pull/24271>
