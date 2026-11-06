---
title: Integration with built-in virtualization
permalink: en/admin/integrations/virtualization/dvp/services.html
---

Deckhouse Platform integrates with the built-in virtualization infrastructure and uses [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass) resources to define the characteristics of virtual machines created as part of the cluster.

Key features:

- Management of built-in virtualization resources via the `cloud-controller-manager module`
- Provisioning of disks using the CSI storage component
- Integration with the [`node-manager`](/modules/node-manager/) module to support DVPInstanceClass when defining a [NodeGroup](/modules/node-manager/cr.html#nodegroup)

{% alert level="info" %}
The virtualization integration is enabled automatically for all [cloud clusters](../../../../glossary.html#cloud-cluster) deployed in built-in virtualization.
No additional configuration is required.
{% endalert %}

## Load balancers

Virtualization supports Service objects of the `LoadBalancer` type. For every such object, [`cloud-controller-manager`](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-dvp.html#module-interactions) creates a `LoadBalancer` Service in the parent cluster and directs traffic to the nodes of the child one. The object is created in the namespace set by the [`namespace`](/modules/cloud-provider-dvp/configuration.html#parameters-provider-parameters-namespace) parameter, next to the cluster virtual machines.

The external address is assigned by the parent cluster, so a balancer controller must run there, for example the [`metallb`](/modules/metallb/) module. The controller serves only those Service objects that have a resolved balancer class.

To set the default class, create a [MetalLoadBalancerClass](/modules/metallb/cr.html#metalloadbalancerclass) resource in the parent cluster with the [`isDefault`](/modules/metallb/cr.html#metalloadbalancerclass-v1alpha1-spec-isdefault) parameter set to `true`. Such a class serves the Service objects with no class specified, so you can leave the class out in the child cluster.

To pick another class, set it in the [`spec.loadBalancerClass`](https://kubernetes.io/docs/concepts/services-networking/service/#load-balancer-class) parameter of the Service object in the child cluster. The platform copies the value to the parent cluster along with the [`externalIPs`](https://kubernetes.io/docs/concepts/services-networking/service/#external-ips) parameter and the deprecated `loadBalancerIP` parameter.

{% alert level="warning" %}
The [`spec.loadBalancerClass`](https://kubernetes.io/docs/concepts/services-networking/service/#load-balancer-class) parameter is immutable. Set the class when creating the Service object, otherwise the object has to be recreated.
{% endalert %}

With no balancer class, the Service object stays in the `Pending` state, and `cloud-controller-manager` reports no errors. Check the state of the object in the parent cluster:

```shell
d8 k -n <NAMESPACE> get svc
```

Replace `<NAMESPACE>` with the namespace set by the [`namespace`](/modules/cloud-provider-dvp/configuration.html#parameters-provider-parameters-namespace) parameter.

### Load balancer network policy

If the [`networkPolicy`](/modules/cloud-provider-dvp/configuration.html#parameters-provider-parameters-networkpolicy) parameter is set to `Isolated`, the parent cluster restricts ingress traffic to the cluster virtual machines (VMs). This value is used by default. To let the balancer traffic reach the nodes, `cloud-controller-manager` creates a NetworkPolicy resource for every balancer in the same namespace of the parent cluster.

The policy allows ingress traffic from any source to the NodePort ports of the Service object in the child cluster over the protocols specified in this object. The policy applies only to the VMs the balancer directs traffic to, so every balancer has its own policy, even if several balancers use the same nodes.

`cloud-controller-manager` overwrites manual changes to the policy the next time it updates the balancer. The policy is deleted together with the balancer Service object in the parent cluster.

To find the balancer policy, follow these steps:

1. Get the UID of the Service object in the child cluster:

   ```shell
   d8 k -n <SERVICE_NAMESPACE> get svc <SERVICE_NAME> -o jsonpath='{.metadata.uid}'
   ```

   Replace `<SERVICE_NAMESPACE>` with the namespace of the Service object and `<SERVICE_NAME>` with its name.

1. Display the policy in the parent cluster:

   ```shell
   d8 k -n <NAMESPACE> get networkpolicy lb-<SERVICE_UID>
   ```

   Replace `<NAMESPACE>` with the namespace set by the [`namespace`](/modules/cloud-provider-dvp/configuration.html#parameters-provider-parameters-namespace) parameter and `<SERVICE_UID>` with the UID obtained in the previous step.
