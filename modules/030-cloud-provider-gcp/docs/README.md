---
title: "Cloud provider — GCP"
description: "Cloud resource management in Deckhouse Kubernetes Platform using Google Cloud Platform."
---

The `cloud-provider-gcp` module integrates Deckhouse Kubernetes Platform with [Google Cloud Platform](https://cloud.google.com/). It allows the [`node-manager`](/modules/node-manager/) module to use GCP resources when provisioning nodes for a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Features of the `cloud-provider-gcp` module:

- Managing GCP resources via `cloud-controller-manager`:
  - creates network routes for the `PodNetwork` network on the GCP side;
  - creates load balancers for Services of the LoadBalancer type;
  - updates cluster node metadata and removes from Kubernetes nodes that no longer exist in GCP.
- Provisioning disks via the Persistent Disk CSI driver (`pd.csi.storage.gke.io`) and creating StorageClasses for GCP disk types so that PersistentVolumes can be requested from the cluster.
- Provisioning CloudEphemeral nodes via Machine Controller Manager (MCM). Virtual machine parameters are set in the [GCPInstanceClass](cr.html#gcpinstanceclass) resource.
- Registering with [`node-manager`](/modules/node-manager/) so that GCPInstanceClass can be used when describing a NodeGroup.
- Enabling CNI for new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used.
