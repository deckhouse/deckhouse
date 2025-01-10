---
title: "Cloud provider — GCP"
---

The `cloud-provider-gcp` module is responsible for interacting with the [Google](https://cloud.google.com/) cloud resources. It allows the [node manager](../../modules/node-manager/) module to use GCP resources for provisioning nodes for the specified [node group](../../modules/node-manager/cr.html#nodegroup) (a group of nodes that are acted upon as if they were a single entity).

The `cloud-provider-gcp` module:
- Manages GCP resources using the `cloud-controller-manager` (CCM) module:
  * The CCM module creates network routes for the `PodNetwork` network on the GCP side.
  * The CCM module creates LoadBalancers for Kubernetes Service objects that have the `LoadBalancer` type.
  * The CCM module updates the metadata of the cluster nodes according to the configuration parameters and deletes nodes that are no longer in GCP.
- Provisions disks in GCP using the `CSI storage` component.
- Enables the necessary CNI plugin (using the [simple bridge](../../modules/cni-simple-bridge/)).
- Registers with the [node-manager](../../modules/node-manager/) module so that [GCPInstanceClasses](cr.html#gcpinstanceclass) can be used when creating the [NodeGroup](../../modules/node-manager/cr.html#nodegroup).
