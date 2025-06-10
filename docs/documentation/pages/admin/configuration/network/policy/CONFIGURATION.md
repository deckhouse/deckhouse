---
title: "Network policy configuration"
permalink: en/admin/configuration/network/policy/configuration.html
---

To define cluster-wide network policies in Deckhouse Kubernetes Platform, you can use the CiliumClusterwideNetworkPolicies module [Cilium](../../reference/mc/cni-cilium/).

<!-- Transferred with minor modifications from https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/cni-cilium/#using-ciliumclusterwidenetworkpolicies -->

{% alert level="danger" %}
Using CiliumClusterwideNetworkPolicies if the `policyAuditMode` option is absent in the cni-cilium module settings may lead to incorrect operation of Control plane or loss of SSH access to all cluster nodes.
{% endalert %}

Follow these steps to use CiliumClusterwideNetworkPolicies:

1. Apply the primary set of `CiliumClusterwideNetworkPolicy` objects. To do this, in the settings of the cni-cilium module add the configuration option [`policyAuditMode`](../cni-cilium/configuration.html#parameters-policyauditmode) with the value `true`.
The option can be removed after applying all `CiliumClusterwideNetworkPolicy` objects and verifying their functionality in Hubble UI.

1. Apply network security policy rule:

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
