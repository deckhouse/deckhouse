---
title: "Network policy configuration"
permalink: en/admin/configuration/network/policy/configuration.html
description: |
  Setting up network policies for the Deckhouse Kubernetes Platform. Conditions for enabling pod traffic tunneling modes.
---

If there are requirements to restrict network interaction between servers at the infrastructure level where the Deckhouse Kubernetes Platform operates, the following conditions must be met:

* Tunneling mode for traffic between pods is enabled ([configuration](/modules/cni-cilium/configuration.html#parameters-tunnelmode) for CNI Cilium, [configuration](/modules/cni-flannel/configuration.html#parameters-podnetworkmode) for CNI Flannel).
* Traffic between [`podSubnetCIDR`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-podsubnetcidr) encapsulated within a VXLAN is allowed (if inspection and filtering of traffic within a VXLAN tunnel is performed).
* If there is integration with external systems (such as LDAP, SMTP, or other external APIs), it is required to allow network communication with them.
* Local network communication is fully allowed within each individual cluster node.
* Inter-node communication is allowed on the ports shown in the [platform component network interaction list](../../../../reference/network_interaction.html). Note that most ports are in the 4200-4299 range. When new platform components are added, they will be assigned ports from this range (if it is possible).

## Configuring network policies via standard Kubernetes means

### Example network policy configuration

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: test-network-policy
  namespace: default
spec:
  podSelector:
    matchLabels:
      app: db
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - ipBlock:
        cidr: 172.17.0.0/16
        except:
        - 172.17.1.0/24
    - namespaceSelector:
        matchLabels:
          project: myproject
    - podSelector:
        matchLabels:
          role: frontend
    ports:
    - protocol: TCP
      port: 6379
  egress:
  - to:
    - ipBlock:
        cidr: 10.0.0.0/24
    ports:
    - protocol: TCP
      port: 5978
```

## Configuring cluster-wide network policies using CiliumClusterwideNetworkPolicy

To define cluster-wide network policies in Deckhouse Kubernetes Platform, you can use the CiliumClusterwideNetworkPolicy objects of the [`cni-cilium`](/modules/cni-cilium/) module.

{% alert level="danger" %}
Using CiliumClusterwideNetworkPolicies while the `policyAuditMode` option is not enabled in the `cni-cilium` module configuration may lead to incorrect operation of the control plane or loss of SSH access to all cluster nodes.
{% endalert %}

To use CiliumClusterwideNetworkPolicies, follow these steps:

1. Apply the primary set of CiliumClusterwideNetworkPolicy objects. To do this, in the `cni-cilium` module configuration, add the option [`policyAuditMode`](/modules/cni-cilium/configuration.html#parameters-policyauditmode) and set it to `true`.

   The `policyAuditMode` option can be removed after applying all CiliumClusterwideNetworkPolicy objects and verifying their functionality in Hubble UI.

1. Apply the network security policy rule:

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

If CiliumClusterwideNetworkPolicies are not used, the control plane may work incorrectly for up to a minute during the reboot of `cilium-agent` Pods. This occurs due to [Conntrack table reset](https://github.com/cilium/cilium/issues/19367). Binding to the `kube-apiserver` entity helps to bypass the bug.
