---
title: "Cloud provider — AWS"
---

The `cloud-provider-aws` module is responsible for interacting with the [AWS](https://aws.amazon.com/) cloud resources. It allows the [node manager](../../modules/040-node-manager/) module to use AWS resources for provisioning nodes for the specified [node group](../../modules/040-node-manager/cr.html#nodegroup) (a group of nodes that are acted upon as if they were a single entity).

The `cloud-provider-aws` module:
- Manages AWS resources using the `cloud-controller-manager` module:
  * It creates network routes for the `PodNetwork` network on the AWS side.
  * It creates LoadBalancers for Kubernetes Service objects that have the `LoadBalancer` type.
  * It updates the metadata of the cluster nodes according to the configuration parameters and deletes nodes that are no longer in AWS.
- Provisions volumes in AWS using the `CSI storage` component.
- Enables the necessary CNI plugin (using the [simple bridge](../../modules/035-cni-simple-bridge/)).
- Registers with the [node-manager](../../modules/040-node-manager/) module so that [AWSInstanceClasses](cr.html#awsinstanceclass) can be used when creating the [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).
