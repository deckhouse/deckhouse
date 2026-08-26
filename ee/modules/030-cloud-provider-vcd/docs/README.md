---
title: "Cloud provider — VMware Cloud Director"
description: "Cloud resource management in Deckhouse Kubernetes Platform using VMware Cloud Director."
---

The `cloud-provider-vcd` module integrates Deckhouse Kubernetes Platform with [VMware Cloud Director](https://www.vmware.com/products/cloud-director.html). It allows the [`node-manager`](/modules/node-manager/) module to use VCD resources when provisioning nodes for a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Features of the `cloud-provider-vcd` module:

- Managing VCD resources via `cloud-controller-manager`:
  - updates virtual machine and Kubernetes node metadata and removes from Kubernetes nodes that no longer exist in VCD;
  - creates load balancers for Services of the LoadBalancer type. This uses VMware NSX Advanced Load Balancer (Avi); support is available with NSX-T.
- Provisioning disks via the Named Disk CSI driver (`named-disk.csi.cloud-director.vmware.com`) so that PersistentVolumes can be requested from the cluster.
- Provisioning CloudPermanent nodes using the [Terraform/OpenTofu provider](/products/kubernetes-platform/documentation/v1/architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-vcd.html#module-interactions) `vmware/vcd`. Base infrastructure is provisioned depending on the [layout](/modules/cloud-provider-vcd/layouts.html).
- Provisioning CloudEphemeral nodes via Cluster API (CAPI). Virtual machine parameters are set in the [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass) resource.
- Registering with [`node-manager`](/modules/node-manager/) so that [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass) can be used when describing a [NodeGroup](/modules/node-manager/cr.html#nodegroup).
- Enabling CNI for new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used.

{% alert level="info" %}
For VCD API versions earlier than 37.2, the module uses compatibility mode (legacy components).
{% endalert %}
