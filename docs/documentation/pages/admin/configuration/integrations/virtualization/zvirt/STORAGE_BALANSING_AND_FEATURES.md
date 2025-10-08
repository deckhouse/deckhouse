---
title: Storage and load balancing
permalink: en/admin/integrations/virtualization/zvirt/storage.html
---

## Storage

In a cluster hosted on a zVirt infrastructure, Storage Domains are used within the scope of the specified [`clusterID`](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration-clusterid).
All virtual machine disks are created within the designated storage domain.

### Requirements

- The [`storageDomainID`](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration-masternodegroup-instanceclass-storagedomainid) specified in the configuration must be available to the `clusterID`
  defined in the [ZvirtClusterConfiguration](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration) resource.
- Disks are created based on the specified [`template`](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration-masternodegroup-instanceclass-template) and placed in the storage domain.
- For PersistentVolume provisioning, VM root disks are used.
  Separate PVCs are not yet supported in zVirt.

### Configuration

Example fragment of ZvirtClusterConfiguration with storage domain reference:

```yaml
masterNodeGroup:
  replicas: 1
  instanceClass:
    numCPUs: 4
    memory: 8192
    rootDiskSizeGb: 40
    template: ALT-p10
    vnicProfileID: "49bb4594-0cd4-4eb7-8288-8594eafd5a86"
    storageDomainID: "c4bf82a5-b803-40c3-9f6c-b9398378f424"
```

{% alert level="info" %}
Use unique identifiers (UUIDs) to specify the template and storage domain.
You can retrieve them through the zVirt API or administrator interface.
{% endalert %}

## Load balancing

The zVirt platform does not provide a built-in load balancer.
To handle incoming traffic, the following approaches are recommended:

1. Use an external load balancer.
   If your infrastructure already has a hardware or software load balancer,
   configure port forwarding (for example, `80/443`) to the cluster's frontend nodes.
1. Use MetalLB.
   For fault-tolerant load balancing within the cluster, you can use MetalLB in Layer 2 (L2) mode.

Recommendations:

- Allocate a dedicated L2 network with DHCP and internet access.
- Define an IP address pool that MetalLB can use for announcements.
- Ensure this network is accessible from the cluster's frontend nodes.
- Leave the network interface configuration empty in the VirtualMachine Template.
  Deckhouse Kubernetes Platform will automatically attach them during VM creation.

{% alert level="info" %}
Support for MetalLB in BGP mode is not guaranteed in zVirt and depends on your network infrastructure.
{% endalert %}
