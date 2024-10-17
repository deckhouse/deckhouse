---
title: "The cni-cilium module"
description: Deckhouse cni-cilium module provides a network between multiple nodes in a Kubernetes cluster using Cilium.
---

This module is responsible for providing a network between multiple nodes in a cluster using the [cilium](https://cilium.io/) module.

## Limitations

1. Service types `NodePort` and `LoadBalancer` do not work with hostNetwork endpoints in the `DSR` LB mode. Switch to `SNAT` if it is required.
2. `HostPort` Pods will bind only to [one interface IP](https://github.com/deckhouse/deckhouse/issues/3035). If there are multiple interfaces/IPs present, Cilium will select only one of them, preferring private IP space.
3. Kernel requirements.
   * The `cni-cilium` module requires a Linux kernel version >= `5.7`.
   * For the `cni-cilium` module to work together with the [istio](../110-istio/), [openvpn](../500-openvpn/) or [node-local-dns]({% if site.d8Revision == 'CE' %}{{ site.urls.ru}}/products/kubernetes-platform/documentation/v1/modules/{% else %}..{% endif %}/350-node-local-dns/) module, a Linux kernel version >= `5.7` is required.
4. OS compatibility issues:
    * Ubuntu:
      * not working on 18.04
      * to work on 20.04 you need to install HWE kernel
    * CentOS:
      * 7 (needs new kernel from [repository](http://elrepo.org))
      * 8 (needs new kernel from [repository](http://elrepo.org))

## A note on handling external traffic in different `bpfLB` modes (replacement for cilium's kube-proxy)
To correctly select the `bpfLB` operating mode, it is important to understand the features and prerequisites for creating each type:
* `SNAT` (Source Network Address Translation) - is one of the NAT subtypes in which for each outgoing packet, the source IP address is translated to the gateway IP address from the target subnet, and incoming packets passing through the gateway are translated back based on the translation table.
* `DSR` (Direct Server Return) - is a load balancing method that allows all incoming traffic to go through the load balancer, and all outgoing traffic to bypass it.  
Kubernetes usually uses schemes where traffic comes to the balancer, which distributes it between many terminating servers. In this case, both incoming and outgoing traffic go through the balancer. Thus, the total throughput is limited by the resources and channel width of the balancer.  
To optimize traffic and unload the balancer, the `DSR` mechanism was invented, in which incoming packets go through the balancer, and outgoing ones go directly from the terminating servers. Since responses are usually much larger in size than requests, this approach can significantly increase the overall throughput of the scheme.


![SNAT data flow diagram](../../images/021-cni-cilium/snat.png)
![DSR data flow diagram](../../images/021-cni-cilium/dsr.png)

The module allows [selecting the operating mode](./configuration.html#parameters-bpflbmode), which affects the behavior of `Service` with the `NodePort` and `LoadBalancer` types:
* `SNAT` (Source Network Address Translation) - in this mode, `bpfLB` completely repeats the logic of `kube-proxy`:
  * if `Service` specifies `externalTrafficPolicy: Local`, then traffic will be transmitted and balanced only to those target pods that are running on the same node to which this traffic arrived. If the target pod is not running on this node, then the traffic will be dropped.
  * if `Service` specifies `externalTrafficPolicy: Cluster`, then traffic will be transmitted and balanced to all target pods in the cluster. In this case, if the target pods are on other nodes, then SNAT will be performed when transmitting traffic to them (the source IP address will be replaced with the InternalIP of the node).
* `DSR` - in this mode, when forwarding traffic to another node, the `DSR` mechanism is used instead of `SNAT`, in which incoming and outgoing traffic goes along asymmetric paths:
  * if `externalTrafficPolicy: Local` is specified in `Service`, then the behavior is absolutely identical to `kube-proxy` and `bpfLB` in `SNAT` mode
  * if `externalTrafficPolicy: Cluster` is specified in `Service`, then the traffic will also be transmitted and balanced to all target pods in the cluster.  
  In this case:
    * if the target pods are on other nodes, then the source IP address will be preserved when incoming traffic is sent to them
    * outgoing traffic will go directly from the node on which the target pod was launched
    * the source IP address will be replaced with the external IP address of the node to which the incoming request **originally** came.
  
{% alert level="warning" %}
In case of using `DSR` and `Service` mode with `externalTrafficPolicy: Cluster` additional network environment settings are required.  
Network equipment must be ready for asymmetric traffic flow: IP address anti-spoofing tools (`uRPF`, `sourceGuard`, etc.) must be disabled or configured accordingly.
{% endalert %}

* `Hybrid` - in this mode, TCP traffic is processed in `DSR` mode, and UDP - in `SNAT` mode.

## A note about CiliumClusterwideNetworkPolicies

1. Make sure that you deploy initial set of CiliumClusterwideNetworkPolicies with `policyAuditMode` configuration options set to `true`.
   Otherwise you are degrading cluster operation or even completely losing SSH connectivity to all Kubernetes Nodes.
   You can remove the option once all `CiliumClusterwideNetworkPolicy` objects are applied and you've verified their effect in the Hubble UI.
2. Make sure to deploy the following rule, otherwise control-plane will fail for up to 1 minute on `cilium-agent` restart. This happens due to [conntrack table reset](https://github.com/cilium/cilium/issues/19367). Referencing `kube-apiserver` entity helps us to "circumvent" the bug.

   ```yaml
   apiVersion: "cilium.io/v2"
   kind: CiliumClusterwideNetworkPolicy
   metadata:
     name: "allow-control-plane-connectivity"
   spec:
     ingress:
     - fromEntities:
       - kube-apiserver
     nodeSelector:
       matchLabels:
         node-role.kubernetes.io/control-plane: ""
   ```

## A note about Cilium work mode change

If you change the Cilium operating mode (the [tunnelMode](configuration.html#parameters-tunnelmode) parameter) from `Disabled` to `VXLAN` or vice versa, you must restart all nodes, otherwise there may be problems with the availability of Pods.

## A note about disabling the kube-proxy module

Cilium has the same functionality as the `kube-proxy` module, so the latter is automatically disabled when the `cni-cilium` module is enabled.

## A note about fault-tolerant Egress Gateway

{% alert level="warning" %} Feature is only available in Enterprise Edition {% endalert %}

### Basic mode

Using pre-configured public IPs of egress-gateway nodes.

<div data-presentation="../../presentations/021-cni-cilium/egressgateway_base_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1Gp8b82WQQnYr6te_zBROKnKmBicdhtX4SXNXDh3lB6Q/ --->

### Virtual IP mode

Allows you to dynamically assign additional IP addresses to nodes.

<div data-presentation="../../presentations/021-cni-cilium/egressgateway_virtualip_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1jdn39uDFSraQIXVdrREBsRv-Lp4kPidhx4C-gvv1DVk/ --->
