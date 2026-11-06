---
title: bpfLB
description: The BPF Load Balancer mode of the [`cni-cilium`](/modules/cni-cilium/) module that processes external traffic for Services of the NodePort and LoadBalancer types. The mode is set by the bpfLBMode parameter.
lang: en
---

In the `SNAT` mode, the BPF load balancer fully replicates the logic of `kube-proxy`: the source IP address of packets sent to pods on other nodes is replaced with the InternalIP of the node. In the [DSR](glossary.html#dsr) mode, incoming traffic passes through the load balancer and outgoing traffic goes directly from the node of the target pod, which increases the overall throughput.

For details, see [BpfLB operation modes](admin/configuration/network/other/bpflb.html).
