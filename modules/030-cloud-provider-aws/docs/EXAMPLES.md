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

1. `service.beta.kubernetes.io/aws-load-balancer-type` — if it has the `none` value, then the target group will **only** be created (without any LoadBalancer).
2. `service.beta.kubernetes.io/aws-load-balancer-backend-protocol` — this parameter is used together with `service.beta.kubernetes.io/aws-load-balancer-type: none`:
   * Possible values:
     * `tcp` (default);
     * `tls`;
     * `http`;
     * `https`.
   * **Caution!** The `cloud-controller-manager` (CCM) will try to recreate the target group in response to changes in this field. If the target group has NLB or ALB attached to it, the CCM will fail to delete it and get stuck in this state forever.  You have to manually disconnect NLB or ALB from the target group.

## Configuring security policies on nodes

### Default security groups

Unless [`disableDefaultSecurityGroup`](cluster_configuration.html#awsclusterconfiguration-disabledefaultsecuritygroup) is enabled (default is disabled), the module creates the following security groups:

* `<prefix>-node` — assigned to cluster nodes. By default it allows:
  * all outbound traffic to `0.0.0.0/0`;
  * all inbound traffic from the `<prefix>-loadbalancer` group;
  * all inbound traffic from nodes in the same `<prefix>-node` group;
  * ICMP from CIDRs listed in [`publicNetworkAllowList`](cluster_configuration.html#awsclusterconfiguration-publicnetworkallowlist) (default `0.0.0.0/0`).
* `<prefix>-loadbalancer` — used by load balancers. By default it allows:
  * all inbound traffic from CIDRs in `publicNetworkAllowList`;
  * all outbound traffic to the `<prefix>-node` group.
* `<prefix>-ssh-accessible` — created when [`sshAllowList`](cluster_configuration.html#awsclusterconfiguration-sshallowlist) is set; allows TCP/22 from the listed CIDRs (default `0.0.0.0/0`).

Additional ports (for example, HTTP/HTTPS) are not added to these groups through the module configuration. Create a separate security group in AWS and attach it with `additionalSecurityGroups`. You can restrict ICMP and load balancer access with `publicNetworkAllowList`, and SSH access with `sshAllowList`.

### Attaching custom security groups

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
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values cloud-provider-aws -o json \
| jq -r '.cloudProviderAws.internal.zoneToSubnetIdMap'
```
