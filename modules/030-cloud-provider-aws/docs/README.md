---
title: "Cloud provider — AWS"
description: "Cloud resource management in Deckhouse Kubernetes Platform using Amazon AWS."
---

The `cloud-provider-aws` module integrates Deckhouse Kubernetes Platform with [Amazon AWS](https://aws.amazon.com/). It allows the [`node-manager`](/modules/node-manager/) module to use AWS resources when provisioning nodes for a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Features of the `cloud-provider-aws` module:

- Managing AWS resources via `cloud-controller-manager`:
  - creates network routes for the `PodNetwork` network on the AWS side;
  - creates load balancers for Services of the LoadBalancer type;
  - updates cluster node metadata and removes from Kubernetes nodes that no longer exist in AWS.
- Provisioning disks via the EBS CSI driver (`ebs.csi.aws.com`) and creating StorageClasses for AWS volume types so that PersistentVolumes can be requested from the cluster.
- Provisioning CloudEphemeral nodes via Machine Controller Manager (MCM). Virtual machine parameters are set in the [AWSInstanceClass](cr.html#awsinstanceclass) resource.
- Registering with [`node-manager`](/modules/node-manager/) so that AWSInstanceClass can be used when describing a NodeGroup.
- Enabling CNI for new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used in `VXLAN` mode with source IP translation via BPF.
