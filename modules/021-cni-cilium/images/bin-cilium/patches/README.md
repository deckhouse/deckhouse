# Patches

## 001-request-ip.patch

Add the oportunity to request specific IP-address using annotation:

    cni.cilium.io/ipAddress: 10.10.10.10

Upstream <https://github.com/cilium/cilium/pull/24098>
Possible feature for refactoring <https://docs.cilium.io/en/v1.14/network/concepts/ipam/multi-pool/>

## 002-stable-mac.patch

Use predefined MAC-addresses for virtualization workloads

Upstream <https://github.com/cilium/cilium/pull/24100>

## 003-mtu.patch

Set correct MTU value for veth interfaces

Upstream issue <https://github.com/cilium/cilium/issues/23711>

## 004-go-mod.patch

Update go.mod and tidy.

## 005-ebpf-dhcp-server.patch

Added DHCP server(ebpf-implementation) for pods

## 006-add-pod-prioroty-managment.patch

Added label allows you to control priority of pods sharing single IP4 address in cluster

## 007-fix-restoring-cep-for-dead-local-endpoint.patch

Fixed bug when agent restoring cep for dead local endpoint
