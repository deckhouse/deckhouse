---
title: Host firewall
description: A Cilium mode in which network policies apply to the cluster nodes themselves rather than to pods.
lang: en
---

A host firewall is configured with CiliumClusterwideNetworkPolicy and the `nodeSelector` field. Such a policy applies to the host endpoint with the label `reserved:host` and filters the traffic entering and leaving the node, including `hostNetwork` pods. It is available only in clusters with the [`cni-cilium`](/modules/cni-cilium/) module.

Host policies add a filter layer inside the cluster and do not replace infrastructure-level protection such as physical firewalls or cloud security groups. A mistake in them can break SSH access, the control plane, kubelet or etcd, so they are rolled out through the audit mode first. For details, see [Host firewall on nodes](admin/configuration/network/policy/host_firewall.html).
