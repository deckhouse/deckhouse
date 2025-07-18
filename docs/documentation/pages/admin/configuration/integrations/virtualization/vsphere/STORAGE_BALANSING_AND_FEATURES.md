---
title: Storage and load balancing
permalink: en/admin/integrations/virtualization/vsphere/vsphere-storage.html
---

## Storage

The following storage types are used in VMware vSphere for Kubernetes cluster data:

- **Datastores**: Used to store the root disks of virtual machines;
- **CNS disks (Container Native Storage)**: Used for automatic creation of PersistentVolumes via CSI.

Deckhouse Kubernetes Platform (DKP) automatically creates a StorageClass for each Datastore and DatastoreCluster
that is tagged as a `zone`.

You can specify:

- The default StorageClass name (`default`).
- Exclusions via the `exclude` field in a form of a list of names or patterns for StorageClasses
  that should not be created.

Example configuration using ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-vsphere
spec:
  version: 1
  enabled: true
  settings:
    storageClass:
      default: fast-lun102
      exclude:
        - ".*-lun101-.*"
        - slow-lun103
```

### Resizing a volume (PVCs)

DKP supports Online Resize PersistentVolume starting with vSphere 7.0U2.
However, due to CSI and vSphere API specifics, additional steps are required after resizing a PVC:

1. Run `kubectl cordon <node_name>`.
1. Delete the Pod that uses the PVC.
1. Wait for the resize operation to complete:
   - Ensure the PVC no longer has the `Resizing` condition.
   - It's safe to ignore the `FileSystemResizePending` status.
1. Run `kubectl uncordon <node_name>`.

## Load balancing

Options for organizing incoming traffic load balancing:

1. **Via an external load balancer**.
   If your infrastructure includes an external load balancer (for example, NSX-T),
   you can route traffic directly to the cluster's frontend nodes.

1. **Via MetalLB**.
   For fault-tolerant load balancing within the cluster, it is recommended that you use MetalLB in BGP mode.
   In this case:

   - Frontend nodes receive two network interfaces.
   - A dedicated VLAN is required for BGP traffic.
   - The network must provide DHCP and internet access.
   - IP addresses and BGP router ASNs must be specified.
   - A pool of IP addresses to be announced must be defined.

{% alert level="info" %}
Make sure there is connectivity between BGP routers and frontend nodes in the dedicated VLAN.
{% endalert %}
