---
title: Integration with VMware vSphere services
permalink: en/admin/integrations/virtualization/vsphere/services.html
---

Deckhouse Kubernetes Platform integrates with VMware vSphere infrastructure. The specifications of virtual machines created as part of the cluster are described in the [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) resource,
and the cloud infrastructure parameters of the cluster are set in the [VsphereClusterConfiguration](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration) resource.

Key features:

- Provisioning and removal of virtual machines via the vCenter API. How this works for different node types is covered in the [vSphere resource management](#vsphere-resource-management) section.
- Node placement across multiple clusters ([`zones`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-zones)) and datacenters ([`region`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-region)).
- Use of VM templates with `cloud-init`. The image requirements and template preparation are covered in the [Connecting to and authorizing in VMware vSphere](authorization.html#preparing-the-virtual-machine-image) section.
- Support for networks with DHCP, static addressing, and additional interfaces. The resource layout is shown in the [Standard layout](layout.html#standard) section.
- Storage management. Provisioning root disks and PersistentVolume volumes on Datastores and CNS disks is covered in the [Storage](storage.html#storage) section.
- Support for incoming traffic load balancing, covered in the [Load balancing](storage.html#load-balancing) section:
  - Via external load balancers.
  - Via MetalLB (in BGP mode).

{% alert level="info" %}
DKP supports hybrid integration with VMware vSphere. For configuration details, see [Hybrid cluster with vSphere](../../hybrid/vsphere-hybrid.html) section.
{% endalert %}

## vSphere resource management

A cluster on vSphere uses nodes of four types:

- [CloudEphemeral](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html): the platform orders, creates, and deletes such nodes automatically. Their number is set in the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource, and the virtual machine parameters are described in the [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) resource.
- [CloudPermanent](../../../../architecture/cluster-and-infrastructure/node-management/cloud-permanent-nodes.html): permanent nodes whose configuration comes from the [`nodeGroups`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups) section of the VsphereClusterConfiguration resource. Changes are applied with the `dhctl converge` command. The cluster master nodes are of this type.
- [CloudStatic](../../../../architecture/cluster-and-infrastructure/node-management/cloud-static-nodes.html): vSphere virtual machines created manually or by third-party tools and connected to the cluster. They are managed by `cloud-controller-manager`, so such nodes get region and zone metadata, and when a virtual machine is deleted from vSphere, the corresponding Node object is deleted from the cluster.
- [Static](../../../../architecture/cluster-and-infrastructure/node-management/static-nodes.html): nodes on bare metal servers or on virtual machines that are not managed by the cloud provider.

Adding nodes, changing their number, and deleting them are covered in the [Node management basics in Deckhouse](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/node/node-management.html) and [Adding and managing cloud nodes](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/node/cloud-node.html) sections. An example of a node group for vSphere is given in the [Creating a node group](/modules/cloud-provider-vsphere/examples.html#creating-a-node-group) section.

The steps below apply only to clusters on vSphere.

### Removing CloudPermanent nodes in vSphere

Nodes of the [`CloudPermanent`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-permanent-nodes.html) type are created based on the node group configuration specified in the [`nodeGroups`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups) section of the VsphereClusterConfiguration resource.

The [`replicas`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups-replicas) parameter defines the required number of virtual machines in the group. After the configuration is changed, run the `dhctl converge` command to start Terraform and bring the state of virtual machines in VMware vSphere in line with the specified number of replicas.

To reduce the number of nodes in a group, decrease the `replicas` value and run `dhctl converge`.

{% alert level="warning" %}
Do not remove the group definition from the [`nodeGroups`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups) section while the value of the [`replicas`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups-replicas) parameter is greater than zero.

If the group definition is removed before the number of replicas is reduced to zero, the Terraform state may become inconsistent with the state of virtual machines and disks in VMware vSphere. As a result, a subsequent `dhctl converge` run may fail and require manual state recovery.
{% endalert %}

#### Reducing the number of nodes

To reduce the number of nodes in a `CloudPermanent` group:

1. Open the vSphere configuration for editing:

   ```shell
   d8 system edit provider-cluster-configuration
   ```

1. In the [`nodeGroups`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups) section, find the required group and decrease the value of the [`replicas`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups-replicas) parameter to the required number of nodes.

   For example, to reduce the number of nodes in the `worker` group from three to two, set `replicas: 2`:

   ```yaml
   nodeGroups:
   - name: worker
     replicas: 2
     zones:
     - zone-a
     instanceClass:
       numCPUs: 4
       memory: 8192
       template: Templates/ubuntu-24.04
       datastore: datastore-1
       mainNetwork: network-1
   ```

   Save the changes.

1. In the [DKP installer container](/products/kubernetes-platform/documentation/v1/installing/#installing), apply the updated configuration:

   ```shell
   dhctl converge \
     --ssh-host <MASTER-NODE-IP-ADDRESS> \
     --ssh-user <USERNAME> \
     --ssh-agent-private-keys /tmp/.ssh/<PRIVATE-SSH-KEY-NAME>
   ```

   {% alert level="info" %}
   Use an installer container of the same edition and version as the cluster.
   {% endalert %}

1. Wait for `dhctl converge` to complete and check the number of nodes in the group:

   ```shell
   d8 k get nodes -l node.deckhouse.io/group=<GROUP-NAME>
   ```

#### Completely removing a node group

A `CloudPermanent` group must be removed in two stages. First, reduce the number of replicas to zero and wait for the nodes and virtual machines to be removed. You can then remove the group definition from the vSphere configuration.

To completely remove a `CloudPermanent` group:

1. Open the vSphere configuration for editing:

   ```shell
   d8 system edit provider-cluster-configuration
   ```

1. In the [`nodeGroups`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups) section, find the group to be removed and set `replicas: 0`. Do not remove the group definition at this stage. For example:

   ```yaml
   nodeGroups:
   - name: worker
     replicas: 0
     zones:
     - zone-a
     instanceClass:
       numCPUs: 4
       memory: 8192
       template: Templates/ubuntu-24.04
       datastore: datastore-1
       mainNetwork: network-1
   ```

   Save the changes.

1. In the [DKP installer container](/products/kubernetes-platform/documentation/v1/installing/#installing), apply the updated configuration:

   ```shell
   dhctl converge \
     --ssh-host <MASTER-NODE-IP-ADDRESS> \
     --ssh-user <USERNAME> \
     --ssh-agent-private-keys /tmp/.ssh/<PRIVATE-SSH-KEY-NAME>
   ```

1. Wait for `dhctl converge` to complete successfully and make sure that no nodes remain in the group:

   ```shell
   d8 k get nodes -l node.deckhouse.io/group=<GROUP-NAME>
   ```

   The command must not return any nodes from the group being removed.

   Also use vSphere Client to make sure that the virtual machines associated with the group have been removed.

1. Open the cloud provider configuration again:

   ```shell
   d8 system edit provider-cluster-configuration
   ```

1. Remove the group definition from the `nodeGroups` section and save the changes.

1. Apply the configuration again:

   ```shell
   dhctl converge \
     --ssh-host <MASTER-NODE-IP-ADDRESS> \
     --ssh-user <USERNAME> \
     --ssh-agent-private-keys /tmp/.ssh/<PRIVATE-SSH-KEY-NAME>
   ```

1. Make sure that the NodeGroup object has been removed:

   ```shell
   d8 k get nodegroup <GROUP-NAME>
   ```

   The command must return a message stating that the requested resource was not found.
