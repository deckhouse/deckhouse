---
title: Node management in Amazon Web Services
permalink: en/admin/integrations/public/amazon/node-management.html
lang: en
---

## Connecting CloudStatic nodes

To connect manually created EC2 instances to a Deckhouse Platform cluster, follow these steps:

1. Attach the IAM role `<prefix>-node`.
1. Attach the security group `<prefix>-node`.
1. Add the following tags:

   ```yaml
   "kubernetes.io/cluster/<cluster_uuid>" = "shared"
   "kubernetes.io/cluster/<prefix>" = "shared"
   ```

To get the `cluster_uuid`, use the following command:

```shell
d8 k -n kube-system get cm d8-cluster-uuid -o json | jq -r '.data."cluster-uuid"'
```

To get the `prefix`, use the following command:

```shell
d8 k get mc global -o jsonpath='{.spec.settings.prefix}{"\n"}'
```

If the value is empty, get it from the deprecated [`cloud.prefix`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-cloud-prefix) parameter of [ClusterConfiguration](../../../../glossary.html#cluster-configuration) (the prefix has moved to the ModuleConfig `global`):

```shell
d8 k -n kube-system get secret d8-cluster-configuration -o json | \
  jq -r '.data."cluster-configuration.yaml"' | base64 -d | grep prefix
```

## Preemptible nodes

Preemptible virtual machines in AWS are called Spot Instances. To make a [node group](../../../../glossary.html#node-group) order them, set the [`spot`](/modules/cloud-provider-aws/cr.html#awsinstanceclass-v1-spec-spot) parameter of the AWSInstanceClass resource to `true`.

When an [EC2 Instance Rebalance Recommendation](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/rebalance-recommendations.html) signals an elevated risk of Spot Instance interruption, the `node-termination-handler` component automatically applies the `aws-node-termination-handler/rebalance-recommendation` taint with `effect: NoSchedule` to the CloudEphemeral node. This taint prevents the scheduler from placing new workloads on the node unless they have a matching toleration.

The `aws-node-termination-handler/spot-itn` taint indicates a notice of an upcoming [Spot Instance interruption](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-interruptions.html).

If either taint is present, the [`node-manager`](/modules/node-manager/) module cordons and drains the node, then deletes it. If the node count in a zone falls below the [`minPerZone`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-minperzone) parameter value, the group provisions new nodes.

The recommendation allows node decommissioning to start before an interruption notice arrives. AWS may send both notifications at the same time.

How to create such nodes and the limitations of their use are described in ["How to create preemptible nodes in AWS?"](/modules/cloud-provider-aws/faq.html#how-to-create-preemptible-nodes-in-aws).

## Replacing the OS image

The OS image of the nodes is set by the [`ami`](/modules/cloud-provider-aws/cr.html#awsinstanceclass-v1-spec-ami) parameter of the AWSInstanceClass resource for `CloudEphemeral` nodes and in the `instanceClass` section of the AWSClusterConfiguration resource for `CloudPermanent` nodes and master nodes. To move the cluster nodes to a new image, follow the steps from ["How do I replace the OS image in a cloud cluster?"](../../../../faq.html#how-do-i-replace-the-os-image-in-a-cloud-cluster).
