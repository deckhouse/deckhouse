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

## Note on `Service` with `NodePort` type and `LoadBalancer`
The module allows [selection of operation mode](./configuration.html#parameters-bpflbmode), which affects the behavior of `Service` with `NodePort` or `LoadBalancer` type:
* `SNAT` - traffic from the client to the pod (and back) passes through NAT, and accordingly the sender's address is lost.
* `DSR` - traffic from the client to the pod passes with the sender's address preserved, and back - according to the routing rules (bypassing the balancer). This mode saves network traffic and reduces delays, but only works for TCP traffic.
* `Hybrid` - TCP traffic is processed in DSR mode, and UDP traffic is processed in SNAT mode.

When creating a `Service` with the type `NodePort` and `LoadBalancer`, you should also consider the `externalTrafficPolicy` parameter, which is directly related to the Cilium operating mode:
* `externalTrafficPolicy: Cluster` (default value) - all incoming traffic to `NodePort` or `LoadBalancer` will be accepted by any node in the cluster, regardless of which pod the target application is on. If the target pod is not on the same node, the traffic will be redirected to the desired node.
The further behavior depends on the module settings:
  * If the module is used in `SNAT` mode, the original client IP will not be saved, as it will be changed to the node IP.
  * When using the module in `DSR` or `Hybrid` mode, the original IP is preserved, but the node processing the request must have an interface on which the sender's IP address will be available to form a response (i.e. if traffic comes from an interface with a "white" IP, then the end node processing the request must also have an interface with a "white" IP)
* `externalTrafficPolicy: Local` - incoming traffic will be accepted only by those nodes on which the target pod is running. If the target pod is not running on a specific node, all traffic to this node will be discarded.

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
