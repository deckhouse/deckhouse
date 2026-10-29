---
title: Storage and load balancing in Basis Dynamix
permalink: en/admin/integrations/private/dynamix/storage.html
---

## Storage

Virtual machine disk placement in Basis Dynamix is defined by a storage policy. It sets the storage endpoint and pool pairs available to the account, as well as an IOPS limit. Basis Dynamix chooses the exact placement of a disk within the policy.

The [`storagePolicy`](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration-storagepolicy) parameter at the root of DynamixClusterConfiguration sets the storage policy for the disks of all virtual machines in the cluster. To use a different storage policy for a node group, set `storagePolicy` in its `instanceClass`, including `masterNodeGroup.instanceClass`, or in a [DynamixInstanceClass](/modules/cloud-provider-dynamix/cr.html#dynamixinstanceclass-v1-spec-storagepolicy). The [`rootDiskSizeGb`](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration-masternodegroup-instanceclass-rootdisksizegb) parameter sets the root disk size of each virtual machine in gigabytes.

Example configuration:

```yaml
storagePolicy: storage_policy01
masterNodeGroup:
  replicas: 1
  instanceClass:
    rootDiskSizeGb: 50
```

The module creates a StorageClass for every `ENABLED` storage policy. The StorageClass naming rules and the way to exclude StorageClasses from the cluster are described in the [module FAQ](/modules/cloud-provider-dynamix/faq.html#which-storageclasses-does-the-module-create).

Disable a storage policy in Basis Dynamix only when the cluster nodes and volumes no longer use it. When a policy is disabled, the module detaches it from the [cluster resource group](/modules/cloud-provider-dynamix/environment.html#storage-policies-in-the-cluster-resource-group) and deletes its StorageClass.

### Changing the storage policy

The storage policy of a disk is set when the virtual machine is created and doesn't change for an existing machine. Therefore, when the storage policy changes, the module recreates nodes of the [`CloudEphemeral`](/modules/node-manager/cr.html#nodegroup-v1-spec-nodetype) type:

- When `storagePolicy` changes in a DynamixInstanceClass: The nodes of the groups that use this DynamixInstanceClass.
- When `storagePolicy` changes at the root of DynamixClusterConfiguration: The nodes of all groups, including groups with their own storage policy.

The module doesn't recreate master nodes and the nodes of the `nodeGroups` groups. A new storage policy applies to such a node only when the node is created again.

## Load balancing

The Basis Dynamix platform doesn't provide a built-in load balancer. To handle inbound traffic to a Deckhouse Platform cluster, the following approaches are recommended:

1. An external load balancer. If your infrastructure has an external load balancer (hardware or software), configure it to forward ports 80 and 443 to the cluster's frontend nodes.

1. Using MetalLB. You can use MetalLB in L2 mode to provide fault-tolerant load balancing.

Recommendations:

- Allocate a separate L2 network with DHCP and internet access.
- Configure the IP address range MetalLB will announce addresses from.
- Make sure this network is connected to the cluster's frontend nodes.
- Leave the network interfaces empty in the VirtualMachine Template configuration. Deckhouse Platform creates them automatically.

{% alert level="info" %}
Support for BGP mode depends on the network infrastructure and isn't guaranteed on Basis Dynamix.
{% endalert %}

### Configuring a Service of the LoadBalancer type

To configure a Service of the LoadBalancer type, add the following annotations to the Service manifest:

```yaml
metadata:
  annotations:
    dynamix.cpi.flant.com/internal-network-name: <internal_name>
    dynamix.cpi.flant.com/external-network-name: <external_name>
```

Both annotations are required:

- `dynamix.cpi.flant.com/internal-network-name`: The name of the internal network in Basis Dynamix
- `dynamix.cpi.flant.com/external-network-name`: The name of the external network in Basis Dynamix

The terms "internal network" and "external network" are used in the context of Basis Dynamix. The external network doesn't have to be public and may use private IP addresses.

If one of the annotations isn't specified, `cloud-controller-manager` fails to process the Service with an error.
