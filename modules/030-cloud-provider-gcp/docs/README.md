---
title: "Cloud provider — GCP"
description: "Cloud resource management in Deckhouse Kubernetes Platform using Google Cloud Platform."
---

The `cloud-provider-gcp` module is responsible for interacting with the [Google](https://cloud.google.com/) cloud resources. It allows the [node manager](../../modules/node-manager/) module to use GCP resources for provisioning nodes for the specified [node group](../../modules/node-manager/cr.html#nodegroup) (a group of nodes that are acted upon as if they were a single entity).

Features of the `cloud-provider-gcp` module:

- Managing GCP resources using the `cloud-controller-manager` (CCM) module:
  - Creating network routes for the `PodNetwork` network on the GCP side.
  - Creating LoadBalancers for Kubernetes Service objects of the `LoadBalancer` type.
  - Updating cluster node metadata of the cluster nodes according to the configuration parameters and deletes nodes that are no longer in GCP.
- Provisioning disks in GCP using the `CSI storage` component.
- Enabling the necessary CNI plugin (uses the [simple bridge](../../modules/cni-simple-bridge/)).
- Register in the [node-manager](../../modules/node-manager/) module so that [GCPInstanceClasses](cr.html#gcpinstanceclass) can be used when creating the [NodeGroup](../../modules/node-manager/cr.html#nodegroup).
