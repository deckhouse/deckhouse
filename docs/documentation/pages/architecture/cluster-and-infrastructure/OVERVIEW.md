---
title: Cluster & Infrastructure subsystem
permalink: en/architecture/cluster-and-infrastructure/
search: cluster & infrastructure, node management
description: Architecture of the Cluster & Infrastructure subsystem in Deckhouse Kubernetes Platform.
extractedLinksOnlyMax: 0
extractedLinksMax: 0
---

This section describes the architecture of the Cluster & Infrastructure subsystem of the Deckhouse Kubernetes Platform (DKP).

The Cluster & Infrastructure subsystem is responsible for the infrastructure layer of Kubernetes cluster management. Cluster node management is implemented using the [`node-manager`](/modules/node-manager/) module, while interaction with IaaS providers is handled by the corresponding `cloud-provider-` family modules.

This section describes:

* The mechanisms for managing all node types used in DKP, as well as [hybrid node groups and clusters](node-management/hybrid-nodegroups-and-clusters.html).
* The reference [CSI driver](infrastructure/csi-driver.html) architecture used in DKP.
* [Bashible](node-management/bashible.html) service, which is a key component of the Cluster & Infrastructure subsystem. Bashible is used by the [`node-manager`](/modules/node-manager/) module to manage node configuration.

The Cluster & Infrastructure subsystem also includes the following modules:

* [`chrony`](/modules/chrony/): Provides time synchronization across all cluster nodes.
* [`registry-packages-proxy`](/modules/registry-packages-proxy/): Provides an internal proxy server for container registry packages.
* [`terraform-manager`](/modules/terraform-manager/): Provides tools for managing Terraform state within a Kubernetes cluster.
* Modules for cloud providers supported by DKP:

  * [`cloud-provider-aws`](/modules/cloud-provider-aws/)
  * [`cloud-provider-azure`](/modules/cloud-provider-azure/)
  * [`cloud-provider-dvp`](/modules/cloud-provider-dvp/)
  * [`cloud-provider-dynamix`](/modules/cloud-provider-dynamix/)
  * [`cloud-provider-gcp`](/modules/cloud-provider-gcp/)
  * [`cloud-provider-huaweicloud`](/modules/cloud-provider-huaweicloud/)
  * [`cloud-provider-openstack`](/modules/cloud-provider-openstack/)
  * [`cloud-provider-vcd`](/modules/cloud-provider-vcd/)
  * [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/)
  * [`cloud-provider-yandex`](/modules/cloud-provider-yandex/)
  * [`cloud-provider-zvirt`](/modules/cloud-provider-zvirt/)
