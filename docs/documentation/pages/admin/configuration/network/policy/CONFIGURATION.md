---
title: "Network policies"
permalink: en/admin/configuration/network/policy/configuration.html
description: |
  Overview of network policy implementations in Deckhouse Kubernetes Platform: NetworkPolicy, CiliumNetworkPolicy, CiliumClusterwideNetworkPolicy, host firewall.
search: network policy, network policies, NetworkPolicy, CiliumNetworkPolicy, CiliumClusterwideNetworkPolicy, host firewall
---

Network policies restrict how pods communicate with each other, with external systems, and with cluster nodes. In Deckhouse Kubernetes Platform (DKP), the implementation depends on the enabled CNI module.

## Network policy implementation in DKP

The available policy formats and the engine that processes them depend on the enabled CNI module:

- With the [`cni-cilium`](/modules/cni-cilium/) module, the implementation is built into Cilium and supports three policy formats:
  - the standard [`NetworkPolicy`](https://kubernetes.io/docs/concepts/services-networking/network-policies/) at L3 and L4;
  - [`CiliumNetworkPolicy`](https://docs.cilium.io/en/v1.17/network/kubernetes/policy/#ciliumnetworkpolicy) — a namespaced resource with L3–L7 rules;
  - [`CiliumClusterwideNetworkPolicy`](https://docs.cilium.io/en/v1.17/network/kubernetes/policy/#ciliumclusterwidenetworkpolicy) — a cluster-scoped resource that also supports `nodeSelector` for protecting nodes (host firewall).
- With the `cni-flannel` module or another CNI without policy support, the [`network-policy-engine`](/modules/network-policy-engine/) module handles enforcement on top of [kube-router](https://github.com/cloudnativelabs/kube-router). Only the standard `NetworkPolicy` at L3 and L4 is supported. Policies are translated into `iptables` and `ipset` rules on every node.

{% alert level="warning" %}
Do not enable `cni-cilium` and `network-policy-engine` at the same time: Cilium already enforces network policies.
{% endalert %}

## What is available in each engine

When choosing a policy format, consider what each engine supports:

- standard `NetworkPolicy` (L3/L4, namespaced) — supported by both engines;
- `CiliumNetworkPolicy` (L3–L7, FQDN, deny rules, namespaced) — only with `cni-cilium`;
- `CiliumClusterwideNetworkPolicy` (L3–L7, FQDN, deny rules, cluster-scoped) — only with `cni-cilium`;
- node-level host firewall via `CiliumClusterwideNetworkPolicy` with `nodeSelector` — only with `cni-cilium`;
- policy audit mode ([`policyAuditMode`](/modules/cni-cilium/configuration.html#parameters-policyauditmode)) — only with `cni-cilium`.

## Network infrastructure requirements

If the underlying infrastructure restricts network communication between servers, make sure the following conditions are met:

- Pod traffic tunneling is enabled: [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode) for CNI Cilium, [`podNetworkMode`](/modules/cni-flannel/configuration.html#parameters-podnetworkmode) for CNI Flannel.
- Traffic between pod subnets ([`podSubnetCIDR`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-podsubnetcidr)) encapsulated in VXLAN is allowed, if the network inspects traffic inside the tunnel.
- Communication with external systems the cluster integrates with (LDAP, SMTP, external APIs) is allowed.
- Local communication inside each cluster node is allowed.
- Inter-node communication is allowed on the ports listed in the [platform component network interaction list](../../../../reference/network_interaction.html). Most ports are in the 4200–4299 range; new platform components are assigned ports from the same range when possible.

## Sections

- [Kubernetes NetworkPolicy](kubernetes_networkpolicy.html) — isolation model, selectors, default policies, API limitations.
- [CiliumNetworkPolicy and CiliumClusterwideNetworkPolicy](cilium_networkpolicy.html) — Cilium extensions, entities, L7 rules, FQDN rules, audit mode.
- [Host firewall on nodes](host_firewall.html) — protecting nodes with `CiliumClusterwideNetworkPolicy` and `nodeSelector`.
- [Common policy examples](examples.html) — recipes for typical tasks.
- [Diagnostics and observability](troubleshooting.html) — how to verify and debug policies.

## Additional documentation

- [Network Policies — Kubernetes documentation](https://kubernetes.io/docs/concepts/services-networking/network-policies/)
- [Network Policy — Cilium documentation](https://docs.cilium.io/en/v1.17/network/kubernetes/policy/)
- [Overview of Network Policy — Cilium documentation](https://docs.cilium.io/en/v1.17/security/policy/)
- [Host Firewall — Cilium documentation](https://docs.cilium.io/en/v1.17/security/host-firewall/)
