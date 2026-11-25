---
title: Integration with VMware Cloud Director
permalink: en/admin/integrations/virtualization/vcd/overview.html
---

Deckhouse Platform integrates with [VMware Cloud Director](https://www.vmware.com/products/cloud-director.html) (VCD) and provisions cluster nodes in it. Virtual machine parameters are set in the [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass) resource, which you reference in a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Integration features:

- Managing VCD resources:
  - updates virtual machine and cluster node metadata and removes from cluster nodes that no longer exist in VCD;
  - creates load balancers for Services of the LoadBalancer type. This uses VMware NSX Advanced Load Balancer (Avi), and support is available with NSX-T.
- Provisioning disks via the Named Disk CSI driver (`named-disk.csi.cloud-director.vmware.com`) so that PersistentVolumes can be requested from the cluster.
- Provisioning CloudPermanent nodes using the [Terraform/OpenTofu provider](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-vcd.html#module-interactions) `terraform-provider-vcd`. Base infrastructure is provisioned depending on the [layout](./layout.html).
- Provisioning CloudEphemeral nodes via Cluster API (CAPI).
- Enabling CNI in new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used.

{% alert level="info" %}
For VCD API versions earlier than 37.2, the platform uses compatibility mode (legacy components).
{% endalert %}
