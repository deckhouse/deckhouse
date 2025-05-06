---
title: "The cni-cilium module"
description: The cni-cilium module provides networking in a cluster using the Cilium module.
---

The `cni-cilium module` provides a network in a cluster. It is based on the [Cilium](https://cilium.io/) project.

## Limitations

1. Services with type `NodePort` and `LoadBalancer` are incompatible with hostNetwork endpoints in LB mode `DSR`. Switch to `SNAT` mode if it is required.
2. `HostPort` pods only bind to [one IP address](https://github.com/deckhouse/deckhouse/issues/3035). If the OS has multiple ultiple interfaces/IP, Cilium will choose one, preferring `private` to `public`.
3. Kernel requirements:
   * Linux kernel version not lower than `5.7` for the `cni-cilium` module to work and work together with the [istio](../istio/), [openvpn](../openvpn/) or [node-local-dns]({% if site.d8Revision == 'CE' %}{{ site.urls.ru}}/products/kubernetes-platform/documentation/v1/modules/{% else %}..{% endif %}/node-local-dns/) modules.
4. OS compatibility:
    * Ubuntu:
      * incompatible with version 18.04;
      * HWE kernel installation required for working with version 20.04.
    * CentOS:
      * for versions 7 and 8, a new kernel from the [repository](https://elrepo.org) is required.

## Handling external traffic in different `bpfLB` modes (replacing kube-proxy from Cilium)

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

* `Hybrid` — in this mode, TCP traffic is processed in `DSR` mode, and UDP in `SNAT` mode.

## Using CiliumClusterwideNetworkPolicies

To use CiliumClusterwideNetworkPolicies, apply:

1. The primary set of `CiliumClusterwideNetworkPolicy` objects with the configuration option `policyAuditMode` set to `true`.
   The absence of this option may lead to incorrect operation of the control plane or loss of SSH access to all cluster nodes . The option can be removed after applying all `CiliumClusterwideNetworkPolicy` objects and verifying their functionality in Hubble UI.
2. Network security policy rule:

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

If CiliumClusterwideNetworkPolicies are not used, the control plane may work incorrectly for up to a minute during the reboot of `cilium-agent` pods. This occurs due to [Conntrack table reset](https://github.com/cilium/cilium/issues/19367). Binding to the `kube-apiserver` entity helps to bypass the bug.

## Changing Cilium Operation Mode

When changing Cilium's operation mode (the [tunnelMode](configuration.html#parameters-tunnelmode) parameter) from `Disabled` to `VXLAN` or vice versa, it is necessary to reboot all nodes, otherwise, pod availability issues may occur.

## Disabling the kube-proxy Module

Cilium fully replaces the functionality of the `kube-proxy` module, so `kube-proxy` is automatically disabled when the `cni-cilium` module is enabled.

## Using Egress Gateway

{% alert level="warning" %}The feature is available only in the following Deckhouse Kubernetes Platform editions: SE+, EE.{% endalert %}

### Basic mode

Pre-configured IP addresses are used on egress nodes.

<div data-presentation="../../presentations/cni-cilium/egressgateway_base_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1Gp8b82WQQnYr6te_zBROKnKmBicdhtX4SXNXDh3lB6Q/ --->

### Virtual IP mode

The ability to dynamically assign additional IP addresses to nodes is implemented.

<div data-presentation="../../presentations/cni-cilium/egressgateway_virtualip_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1jdn39uDFSraQIXVdrREBsRv-Lp4kPidhx4C-gvv1DVk/ --->
