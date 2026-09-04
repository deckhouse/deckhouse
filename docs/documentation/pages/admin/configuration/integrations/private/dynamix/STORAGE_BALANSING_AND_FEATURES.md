---
title: Storage and load balancing in Basis Dynamix
permalink: en/admin/integrations/private/dynamix/storage.html
---

## Storage

Virtual machine disk placement in the Basis Dynamix cloud is set by the storage policy name:

- [`storagePolicy`](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration-storagepolicy) — the name of the storage policy used by default for the disks of all virtual machines in the cluster. A storage policy defines a set of available storage endpoint + pool pairs and an IOPS limit; the platform chooses the exact placement within the policy. It's set at the root of DynamixClusterConfiguration and can be overridden for an individual instanceClass, including in the `masterNodeGroup.instanceClass` section;
- [`rootDiskSizeGb`](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration-masternodegroup-instanceclass-rootdisksizegb) — the root disk size of each virtual machine (in gigabytes).

Example configuration:

```yaml
storagePolicy: storage_policy01
masterNodeGroup:
  replicas: 1
  instanceClass:
    rootDiskSizeGb: 50
```

{% alert level="info" %}
Set `storagePolicy` on an instanceClass only when that node group must use a storage policy different from the one set at the root of DynamixClusterConfiguration.
{% endalert %}

## Load balancing

The Basis Dynamix platform doesn't provide a built-in load balancer. To handle inbound traffic to a Deckhouse Kubernetes Platform cluster, the following approaches are recommended:

1. An external load balancer. If your infrastructure has an external load balancer (hardware or software), configure it to forward ports 80 and 443 to the cluster's frontend nodes.

1. Using MetalLB. You can use MetalLB in L2 mode to provide fault-tolerant load balancing.

Recommendations:

- Allocate a separate L2 network with DHCP and internet access.
- Configure the IP address range MetalLB will announce addresses from.
- Make sure this network is connected to the cluster's frontend nodes.
- Leave the network interfaces empty in the VirtualMachine Template configuration — Deckhouse will create them automatically.

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

- `dynamix.cpi.flant.com/internal-network-name` — the name of the internal network in Basis Dynamix;
- `dynamix.cpi.flant.com/external-network-name` — the name of the external network in Basis Dynamix.

The terms "internal network" and "external network" are used in the context of Basis Dynamix. The external network doesn't have to be public and may use private IP addresses.

If one of the annotations isn't specified, cloud-controller-manager will fail to process the Service with an error.
