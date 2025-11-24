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

It needs to be changed to <https://docs.cilium.io/en/latest/network/pod-mac-address/#pod-mac-address>

## 003-mtu.patch

Set correct MTU value for veth interfaces

Upstream issue <https://github.com/cilium/cilium/issues/23711>

## 005-ebpf-dhcp-server.patch

Added DHCP server for pods (ebpf implementation).

## 006-add-pod-prioroty-management.patch

Added a `network.deckhouse.io/pod-common-ip-priority` label allows you to share a single IP between  several Pods and to switch the actual owner.

## 007-fix-restoring-cep-for-dead-local-endpoint.patch

Fixed bug when agent uses CiliumEndpoint cache for dead local endpoints.

## 008-hide-error-of-incompatibility-of-egw-with-ces.patch

In the PR <https://github.com/cilium/cilium/pull/27984>, an error has been introduced if `CES` and `EGW` are enabled together, as some of the features are not functioning correctly.

While we were previously satisfied with the older behavior, the agent is now unable to start due to this error.

Please remove this change after `CES` becomes Stable. <https://github.com/cilium/cilium/issues/31904#issuecomment-2647858564>

## 009-wireguard-port.patch

Changing the hardcoded wireguard port from `51871` to `4287` (a port within our range).

## 010-fix-cilium-local-redirect-policy.patch

When the `bpf-lb-algorithm-annotation` option is enabled, the `CiliumLocalRedirectPolicy` in Cilium version 1.17.4 stops working. This patch solves the problem with the way the LoadBalancerAlgorithm processes.

Upstream PR <https://github.com/cilium/cilium/pull/40246>.

## 011-bpf-lb-use-random-lb-algo-for-hostport-serives-fixed.patch

For HostPort pseudo-serivces always use random LB algo. When bpf-lb-algorithm-annotation feature activated - use default LB algo if it incorrectly choosed in service map.

## 012-add-least-conn-lb-algorithm.patch

Added an implementation of the Least Connections load balancing algorithm.

## 013-ignore-egress-gateway-inactual-warning.patch

Ignore error when using IPv4 address for egress gateway other than assigned

## 014-adding-ct-import-of-dump-in-json-format-by-stdin.patch

Added ct dump import

## 015-fixed-endpoint-conntrack-clearing-logic-for-vm-migra.patch

Fixed endpoint clearing logic
