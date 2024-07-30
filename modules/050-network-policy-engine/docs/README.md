---
title: "The network-policy-engine module"
---

<div class="docs__information warning active">
Do not use the module if the <a href="../021-cni-cilium/">cilium</a> module is enabled because it already has network policy management. 
</div>

This module manages network policies.

Deckhouse implements a conservative approach to organizing the network based on elementary network backends, such as *"pure"* CNI or flannel in the `host-gw` mode. This approach is reliable and straightforward and turned out to be the best.

The `NetworkPolicy` implementation in Deckhouse is also solid and straightforward. It is based on `kube-router` in the *Network Policy Controller* mode (`--run-firewall`). In this case, `kube-router` transforms `NetworkPolicy` network policies into `iptables` rules. The latter, in turn, work with any installations (regardless of the cloud or the CNI used).

The `network-policy-engine` module deploys a `d8-system` DaemonSet in the namespace with [kube-router](https://github.com/cloudnativelabs/kube-router) in the [Network Policy Controller](https://kubernetes.io/docs/concepts/services-networking/network-policies/) mode. As a result, the Kubernetes cluster fully supports Network Policies.

The following policy description formats are supported:
- *networking.k8s.io/NetworkPolicy API*
- *network policy V1/GA semantics*
- *network policy beta semantics*

Example recipes are available [here](https://github.com/ahmetb/kubernetes-network-policy-recipes).
