---
title: "The cni-cilium module"
description: Deckhouse cni-cilium module provides a network between multiple nodes in a Kubernetes cluster using Cilium.
---

This module is responsible for providing a network between multiple nodes in a cluster using the [cilium](https://cilium.io/) module.

## Limitations

1. Service types `NodePort` and `LoadBalancer` do not work with hostNetwork endpoints in the `DSR` LB mode. Switch to `SNAT` if it is required.
2. `HostPort` Pods will bind only to [one interface IP](https://github.com/deckhouse/deckhouse/issues/3035). If there are multiple interfaces/IPs present, Cilium will select only one of them, preferring private IP space.
3. Kernel requirements.
   * The `cni-cilium` module requires a Linux kernel version >= `4.9.17`.
   * For the `cni-cilium` module to work together with the [istio](../110-istio/), [openvpn](../500-openvpn/) or [node-local-dns]({% if site.d8Revision == 'CE' %}{{ site.urls.ru}}/documentation/v1/modules/{% else %}..{% endif %}/350-node-local-dns/) module, a Linux kernel version >= `5.7` is required.
4. OS versions support.
   * Ubuntu
     * 18.04
     * 20.04
     * 22.04
   * Debian
     * 11
   * CentOS
     * 7 (requires kernel from external [repo](http://elrepo.org))
     * 8 (requires kernel from external [repo](http://elrepo.org))

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
