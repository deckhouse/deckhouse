# Patches

## 000-go-mod.patch

Fix CVE, update go.mod and go.sum.

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

## 005-ebpf-dhcp-server.patch

Added DHCP server for pods (ebpf implementation).

## 006-add-pod-prioroty-managment.patch

Added a `network.deckhouse.io/pod-common-ip-priority` label allows you to share a single IP between  several Pods and to switch the actual owner.

## 007-fix-restoring-cep-for-dead-local-endpoint.patch

Fixed bug when agent uses CiliumEndpoint cache for dead local endpoints.
