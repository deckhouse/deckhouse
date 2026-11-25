---
title: Integration with Google Cloud Platform (GCP)
permalink: en/admin/integrations/public/gcp/overview.html
---

Deckhouse Platform integrates with [Google Cloud Platform](https://cloud.google.com/) (GCP) and provisions cluster nodes in it. Virtual machine parameters are set in the [GCPInstanceClass](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass) resource, which you reference in a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Integration features:

- Managing GCP resources:
  - creates network routes for the `PodNetwork` network on the GCP side;
  - creates load balancers for Services of the LoadBalancer type;
  - updates cluster node metadata and removes from cluster nodes that no longer exist in GCP.
- Provisioning disks via the Persistent Disk CSI driver (`pd.csi.storage.gke.io`) and creating StorageClasses for GCP disk types so that PersistentVolumes can be requested from the cluster.
- Provisioning base infrastructure and CloudPermanent nodes using the [Terraform/OpenTofu provider](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-gcp.html#module-interactions) `terraform-provider-google`.
- Provisioning CloudEphemeral nodes via Machine Controller Manager (MCM).
- Enabling CNI in new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used.
