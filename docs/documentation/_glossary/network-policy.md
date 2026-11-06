---
title: Network policy
description: Rules that restrict how pods communicate with each other, with external systems and with cluster nodes.
lang: en
---

The implementation of network policies in Deckhouse Platform depends on the enabled CNI module. With [`cni-cilium`](/modules/cni-cilium/), policies are enforced by Cilium in three formats: the standard NetworkPolicy at L3 and L4, Cilium network policies with L3–L7 rules within a namespace, and CiliumClusterwideNetworkPolicy for the whole cluster, which also protects nodes as a [host firewall](glossary.html#host-firewall). With [`cni-flannel`](/modules/cni-flannel/) or another CNI without policy support, the [`network-policy-engine`](/modules/network-policy-engine/) module enforces the standard NetworkPolicy at L3 and L4 through `iptables` and `ipset` rules on every node.

For details, see [Network policies](admin/configuration/network/policy/configuration.html).
