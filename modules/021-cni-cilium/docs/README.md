---
title: "The cni-cilium module"
---

This module is responsible for providing a network between multiple nodes in a cluster using the [cilium](https://cilium.io/) module.

## Limitations

1. This module currently supports only direct-routing mode.
2. Service types `NodePort` and `LoadBalancer` do not work with hostNetwork endpoints in the `DSR` LB mode.
3. OS versions support. `cni-cilium` module will properly work only on Linux kernel >= 5.3
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
