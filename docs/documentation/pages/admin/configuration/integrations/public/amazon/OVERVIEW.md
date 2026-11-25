---
title: Integration with Amazon Web Services (AWS)
permalink: en/admin/integrations/public/amazon/overview.html
---

Deckhouse Platform integrates with [Amazon AWS](https://aws.amazon.com/) and provisions cluster nodes in it. Virtual machine parameters are set in the [AWSInstanceClass](/modules/cloud-provider-aws/cr.html#awsinstanceclass) resource, which you reference in a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Integration features:

- Managing AWS resources:
  - creates network routes for the `PodNetwork` network on the AWS side;
  - creates load balancers for Services of the LoadBalancer type;
  - updates cluster node metadata and removes from cluster nodes that no longer exist in AWS.
- Provisioning disks via the EBS CSI driver (`ebs.csi.aws.com`) and creating StorageClasses for AWS volume types so that PersistentVolumes can be requested from the cluster.
- Provisioning base infrastructure and CloudPermanent nodes using the [Terraform/OpenTofu provider](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-aws.html#module-interactions) `terraform-provider-aws`.
- Provisioning CloudEphemeral nodes via Machine Controller Manager (MCM).
- Enabling CNI in new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used.
