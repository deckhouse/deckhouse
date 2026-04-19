---
title: "BpfLB operation modes for external traffic processing"
permalink: en/virtualization-platform/documentation/admin/platform-management/network/other/bpflb.html
---

You can use the [`cni-cilium`](/modules/cni-cilium/) module
to configure the BPF Load Balancer (bpfLB) mode of operation in Deckhouse Virtualization Platform.

In Kubernetes, traffic typically comes through a load balancer
that distributes incoming requests among servers and processes responses.
Thus, the total throughput is limited by the load balancer resources.

To optimize traffic and unload the balancer, the `DSR` mechanism is used,
in which incoming packets go through the load balancer, and outgoing ones go directly from the terminating servers.
Since responses are usually much larger in size than requests, this approach can significantly increase the overall throughput.

The module allows selecting the mode of operation via the [`bpfLBMode`](/modules/cni-cilium/configuration.html#parameters-bpflbmode) parameter,
which affects the behavior of Services of the `NodePort` and `LoadBalancer` types:

* `SNAT` (Source Network Address Translation): A subtype of NAT in which, for each outgoing packet,
  the source IP address is translated to the IP address of the gateway from the target subnet,
  and incoming packets passing through the gateway are translated back based on a translation table.
  In this mode, `bpfLB` fully replicates the logic of `kube-proxy`:
  * If `externalTrafficPolicy: Local` is specified for a Service object,
    the traffic will be forwarded and balanced only to those target pods running on the same node where the traffic arrived.
    If the target pod is not running on this node, the traffic will be dropped.
  * If `externalTrafficPolicy: Cluster` is specified for a Service object,
    the traffic will be forwarded and balanced to all target pods in the cluster.
    At the same time, if the target pods are located on other nodes, SNAT will be performed when transmitting traffic to them
    (the source IP address will be replaced with the InternalIP of the node).

  ![SNAT data flow diagram](/images/cni-cilium/snat.png)

* `DSR` (Direct Server Return): A method where all incoming traffic passes through the load balancer,
  and all outgoing traffic bypasses it. This method is used instead of `SNAT`.
  Often, responses are much larger than requests, and `DSR` can significantly increase the overall throughput of the scheme:
  * If `externalTrafficPolicy: Local` is specified in the `Service`,
    its behavior is completely analogous to `kube-proxy` and `bpfLB` in `SNAT` mode.
  * If `externalTrafficPolicy: Cluster` is specified in the `Service`,
    the traffic will be forwarded and balanced to all target pods in the cluster.
  Note the following:
    * If the target pods are on other nodes, then the source IP address will be preserved when incoming traffic is sent to them.
    * Outgoing traffic will go directly from the node on which the target pod was launched.
    * The source IP address will be replaced with the external IP address of the node to which the incoming request came originally.

  ![DSR data flow diagram](/images/cni-cilium/dsr.png)

  > In case of using `DSR` and `Service` mode with `externalTrafficPolicy: Cluster`
  > additional network environment settings are required.  
  > Network equipment must be ready for asymmetric traffic flow:
  > IP address anti-spoofing tools (`uRPF`, `sourceGuard`, etc.) must be disabled or configured accordingly.

* `Hybrid`: In this mode, TCP traffic is handled per `DSR` rules, while UDP traffic is handled per `SNAT` rules.

Example of bpfLB operation mode configuration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-cilium
spec:
  settings:
    tunnelMode: VXLAN
    bpfLBMode: SNAT # SNAT mode is selected.
  version: 1
  enabled: true
```
