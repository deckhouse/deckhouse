---
title: Storage and load balancing in VMware vSphere
permalink: en/admin/integrations/virtualization/vsphere/storage.html
---

## Storage

The following storage types are used in VMware vSphere for Kubernetes cluster data:

- **Datastores**: Used to store the root disks of virtual machines;
- **CNS disks (Container Native Storage)**: Used for automatic creation of PersistentVolumes via CSI.

Deckhouse Kubernetes Platform (DKP) automatically creates a StorageClass for each Datastore and DatastoreCluster
that is tagged as a `zone`.

You can specify:

- The default StorageClass name ([`default`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-default)).
- Exclusions via the [`exclude`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-exclude) field in a form of a list of names or patterns for StorageClasses
  that should not be created.

Example configuration using ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-vsphere
spec:
  version: 2
  enabled: true
  settings:
    storageClass:
      default: fast-lun102
      exclude:
        - ".*-lun101-.*"
        - slow-lun103
```

### SPBM storage policies

If storage policies (SPBM, Storage Policy Based Management) are configured in vSphere, DKP discovers them and additionally creates a StorageClass for each combination of a Datastore and a policy. When you order a PersistentVolume through such a StorageClass, vSphere applies the corresponding policy to the volume.

The StorageClass name combines the Datastore name and the policy name. DKP converts it to lowercase, replaces spaces with hyphens, and removes the remaining characters except hyphens and dots. For example, the `lun_1` Datastore and the `Gold Policy` policy produce the `lun1-gold-policy` StorageClass.

For DKP to discover the policies, the vSphere account needs the `StorageProfile.View` privilege.

Policies apply to volumes in any scenario where the `cloud-provider-vsphere` module runs, including a hybrid cluster. You cannot pick a policy for an individual StorageClass, since the StorageClass set is generated automatically.

Limitations:

- StorageClasses with policies are created only for Datastore objects, and are not created for DatastoreCluster objects.
- The [`exclude`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-exclude) parameter is matched against the Datastore name, so it removes both the base StorageClass and all StorageClasses with policies for that Datastore.

### Storage policy for node disks

Disks of the virtual machines created by the installer get their storage policy from the [`storagePolicyID`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-storagepolicyid) parameter of the VsphereClusterConfiguration resource. The parameter takes the ID of an SPBM policy.

The parameter applies to master nodes and to static nodes created by the installer. Nodes ordered through a VsphereInstanceClass do not get the policy from this parameter. In a hybrid cluster, where the VsphereClusterConfiguration resource is not used, you cannot set a storage policy for node disks.

### Resizing a volume (PVCs)

DKP supports Online Resize PersistentVolume starting with vSphere 7.0U2. Due to [specifics](https://github.com/kubernetes-csi/external-resizer/issues/44) of the CSI volume-resizer and the vSphere API, perform the following steps after resizing a PVC:

1. Run `d8 k cordon <node_name>` for the node that hosts the Pod.
1. Delete the Pod that uses the PVC.
1. Wait for the resize operation to complete. The PVC must no longer have the `Resizing` condition, while the `FileSystemResizePending` condition is not an issue.
1. Run `d8 k uncordon <node_name>`.

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

## CSI

The storage subsystem uses CNS disks by default, with support for online resizing.  
Legacy mode with FCD disks is also supported. The subsystem behavior is configured via the [`compatibilityFlag`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-compatibilityflag) parameter.

## Datastore configuration

{% alert %}
You can also tag **Datastore** objects through the **VMware vSphere Client** — follow [Datastore configuration](authorization.html#configuring-datastore-in-vsphere-client) in the connection and authorization guide. The steps below use **`govc` only**.
{% endalert %}

{% alert level="warning" %}
For dynamic `PersistentVolume` provisioning, a `Datastore` must be available on **each** ESXi host (shared datastore).
{% endalert %}

For PersistentVolume to function correctly, the datastore must be accessible from all ESXi hosts.

Assign tags:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName2>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```
