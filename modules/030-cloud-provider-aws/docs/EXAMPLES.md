---
title: "Cloud provider — AWS: examples"
---

## An example of the `AWSInstanceClass` custom resource

Below is a simple example of custom resource `AWSInstanceClass` configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: AWSInstanceClass
metadata:
  name: worker
spec:
  instanceType: t3.large
  ami: ami-040a1551f9c9d11ad
  diskSizeGb: 15
  diskType:  gp2
```

## LoadBalancer

### Service object Annotations

The following parameters are supported in addition to the existing [upstream](https://cloud-provider-aws.sigs.k8s.io/service_controller/) ones:

1. `service.beta.kubernetes.io/aws-load-balancer-type` — if it has the `none` value, then the Target Group will **only** be created (without any LoadBalancer).
2. `service.beta.kubernetes.io/aws-load-balancer-backend-protocol` — this parameter is used together with `service.beta.kubernetes.io/aws-load-balancer-type: none`:
   * Possible values:
     * `tcp` (default);
     * `tls`;
     * `http`;
     * `https`.
   * **Caution!** The `cloud-controller-manager` (CCM) will try to recreate the Target Group in response to changes in this field. If the Target Group has NLB or ALB attached to it, the CCM will fail to delete it and get stuck in this state forever.  You have to manually disconnect NLB or ALB from the Target Group.

## Configuring security policies on nodes

There may be many reasons why you may need to restrict or expand incoming/outgoing traffic on cluster VMs in AWS:

* Allow VMs on a different subnet to connect to cluster nodes.
* Allow connecting to the ports of the static node so that the application can work.
* Restrict access to external resources or other VMs in the cloud for security reasons.

For all this, additional security groups should be used. You can only use security groups that are created in the cloud tentatively.

## Enabling additional security groups on static and master nodes

This parameter can be set either in an existing cluster or when creating one. In both cases, additional security groups are declared in the `AWSClusterConfiguration`:
- for master nodes, in the `additionalSecurityGroups` field of the `masterNodeGroup` section;
- for static nodes, in the `additionalSecurityGroups` field of the `nodeGroups` subsection that corresponds to the target nodeGroup.

The `additionalSecurityGroups` field contains an array of strings with security group names.

## Enabling additional security groups on ephemeral nodes

You have to set the `additionalSecurityGroups` parameter for all [`AWSInstanceClass`](cr.html#awsinstanceclass) that require additional security groups.

## Configuring the load balancer if Ingress nodes are not available in all zones

Set the following annotation for the Service object: `service.beta.kubernetes.io/aws-load-balancer-subnets: subnet-foo, subnet-bar`.

You can get current subnets for a particular installation as follows:

```bash
kubectl -n d8-system exec deploy/deckhouse -c deckhouse -- deckhouse-controller module values cloud-provider-aws -o json \
| jq -r '.cloudProviderAws.internal.zoneToSubnetIdMap'
```
