---
title: "Operation modes of bpfLB for external traffic processing"
permalink: en/admin/network/bpflb-external-traffic.html
---

You can use the [Cilium](#) module to configure the bpfLB (BPF Load Balancer) mode of operation in Deckhouse Kubernetes Platform.

<!-- Transferred with minor modifications from https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/cni-cilium/#handling-external-traffic-in-different-bpflb-modes-replacing-kube-proxy-from-cilium -->

Kubernetes typically uses schemes where traffic comes to a balancer that distributes it among many servers. Both incoming and outgoing traffic passes through the balancer. Thus, the total throughput is limited by the resources and channel width of the balancer. To optimize traffic and unload the balancer, the `DSR` mechanism was invented, in which incoming packets go through the balancer, and outgoing ones go directly from the terminating servers. Since responses are usually much larger in size than requests, this approach can significantly increase the overall throughput of the scheme.

To extend the capabilities, the module allows [selectable mode of operation](configuration.html#parameters-bpflbmode), which affects the behavior of `Service` with the `NodePort` and `LoadBalancer` types:

* `SNAT` (Source Network Address Translation) — is a subtype of NAT in which, for each outgoing packet, the source IP address is translated to the IP address of the gateway from the target subnet, and incoming packets passing through the gateway are translated back based on a translation table. In this mode, `bpfLB` fully replicates the logic of `kube-proxy`:
  * if `externalTrafficPolicy: Local` is specified in the `Service`, the traffic will be forwarded and balanced only to those target pods running on the same node where the traffic arrived. If the target pod is not running on this node, the traffic will be dropped.
  * if `externalTrafficPolicy: Cluster` is specified in the `Service`, the traffic will be forwarded and balanced to all target pods in the cluster. At the same time, if the target pods are located on other nodes, SNAT will be performed when transmitting traffic to them (the source IP address will be replaced with the InternalIP of the node).

   ![SNAT data flow diagram](../../images/cni-cilium/snat.png)

* `DSR` (Direct Server Return) — is a method where all incoming traffic passes through the load balancer, and all outgoing traffic bypasses it. This method is used instead of `SNAT`. Often, responses are much larger than requests, and `DSR` can significantly increase the overall throughput of the scheme:
  * if `externalTrafficPolicy: Local` is specified in the `Service`, its behavior is completely analogous to `kube-proxy` and `bpfLB` in `SNAT` mode.
  * if `externalTrafficPolicy: Cluster` is specified in the `Service`, the traffic will be forwarded and balanced to all target pods in the cluster.
  It is important to take into account the following features:
    * if the target pods are on other nodes, then the source IP address will be preserved when incoming traffic is sent to them;
    * outgoing traffic will go directly from the node on which the target pod was launched;
    * the source IP address will be replaced with the external IP address of the node to which the incoming request **originally** came.

   ![DSR data flow diagram](../../images/cni-cilium/dsr.png)

{% alert level="warning" %}
In case of using `DSR` and `Service` mode with `externalTrafficPolicy: Cluster` additional network environment settings are required.  
Network equipment must be ready for asymmetric traffic flow: IP address anti-spoofing tools (`uRPF`, `sourceGuard`, etc.) must be disabled or configured accordingly.
{% endalert %}
