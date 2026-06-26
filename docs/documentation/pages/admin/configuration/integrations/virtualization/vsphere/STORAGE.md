---
title: Storage and load balancing in VMware vSphere
permalink: en/admin/integrations/virtualization/vsphere/storage.html
---

## Overview

In a Deckhouse Kubernetes Platform (DKP) cluster on VMware vSphere, two independent storage types are used:

| Purpose | Technology | Configuration |
|---------|------------|---------------|
| Root disks of virtual machines (cluster nodes) | VM files on a Datastore | [`datastore`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-masterinstanceclass-datastore) in `VsphereClusterConfiguration` / [`VsphereInstanceClass`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) |
| PersistentVolumes for applications | CNS disks (Container Native Storage) via CSI | Automatically via Datastore tags; settings in the [`ModuleConfig`](/modules/cloud-provider-vsphere/configuration.html) of the `cloud-provider-vsphere` module |

A node root disk and an application volume can be placed on the same Datastore or on different ones — they are configured independently.

{% alert level="info" %}
Datastore preparation (tags, ESXi availability) is described in [Connection and authorization](authorization.html#configuring-datastore-in-vsphere-client). Below is cluster-side Kubernetes storage configuration.
{% endalert %}

## Virtual machine root disks

When creating nodes, DKP clones a VM template and places the root disk on the Datastore specified in the node group configuration:

```yaml
instanceClass:
  datastore: dev/lun_1   # path relative to the Datacenter
  rootDiskSize: 50       # root disk size in GiB (optional)
```

Additional parameters:

- [`storagePolicyID`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-storagepolicyid) — SPBM (Storage Policy Based Management) policy ID for root disks of all cluster nodes. When set, vSphere applies the policy to VM disks regardless of the provisioning type.
- DKP creates disks with the `eagerZeroedThick` type, but the final type may be changed by the vSphere storage policy.

{% alert %}
For VM template preparation and disk policies, see [Connection and authorization](authorization.html#virtual-machine-image-requirements).
{% endalert %}

## CSI and PersistentVolumes

### How automatic storage discovery works

The `cloud-data-discoverer` component periodically polls vCenter and builds a list of available Datastores. An object is included if it:

1. Is in a Datacenter tagged with the region tag (category [`regionTagCategory`](/modules/cloud-provider-vsphere/configuration.html#parameters-regiontagcategory), default `k8s-region`).
2. Has a zone tag (category [`zoneTagCategory`](/modules/cloud-provider-vsphere/configuration.html#parameters-zonetagcategory), default `k8s-zone`) from the cluster zone list.
3. Is available on all ESXi hosts in the zone (shared datastore).

Based on discovered Datastores, the `cloud-provider-vsphere` module creates `StorageClass` objects in the Kubernetes cluster.

### StorageClass names

A StorageClass name is derived from the Datastore inventory path: characters are lowercased, spaces are replaced with hyphens. For example, Datastore `dev/lun_102` may become StorageClass `dev-lun-102`.

If VM Storage Policies are configured in vCenter, a separate StorageClass is created for each "Datastore + policy" combination with a name like `<datastore>-<policy-name>`.

### Datastore and DatastoreCluster

Both individual Datastores and Datastore Clusters are discovered. However, StorageClass creation depends on the CSI driver mode:

| vSphere object type | CNS (default mode) | Legacy (FCD) |
|---------------------|-------------------|--------------|
| Datastore | StorageClass is created | StorageClass is created |
| DatastoreCluster | StorageClass is **not** created | StorageClass is created |

For dynamic PVC provisioning in the standard (CNS) mode, use individual Datastores with correct zone tags.

### StorageClass parameters

Created StorageClasses have the following characteristics:

- **Provisioner:** `csi.vsphere.vmware.com` (CNS) or `vsphere.csi.vmware.com` (Legacy).
- **volumeBindingMode:** `WaitForFirstConsumer` (CNS) / `Immediate` (Legacy) — the volume is created on the ESXi host where the Pod is scheduled.
- **allowVolumeExpansion:** `true` — PVC size increase is supported (CNS mode, vSphere 7.0U2+).
- **allowedTopologies:** zone constraints — a PVC is created only on a Datastore tagged with the matching zone.

Example of a created StorageClass (CNS):

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: dev-lun-102
provisioner: csi.vsphere.vmware.com
parameters:
  DatastoreURL: "ds:///vmfs/volumes/..."
  StoragePolicyName: "Gold Policy"   # if a policy is set
allowedTopologies:
- matchLabelExpressions:
  - key: failure-domain.beta.kubernetes.io/region
    values: ["test-region"]
  - key: failure-domain.beta.kubernetes.io/zone
    values: ["test-zone-1"]
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
```

### Configuring StorageClasses in the cluster

Via the `cloud-provider-vsphere` module `ModuleConfig` you can:

- **Exclude** unwanted StorageClasses — parameter [`storageClass.exclude`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-exclude). Accepts exact names or regex expressions.
- **Set the default StorageClass** — use the global parameter [`global.defaultClusterStorageClass`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-defaultclusterstorageclass). The module parameter `storageClass.default` is deprecated.

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
      exclude:
        - ".*-lun101-.*"
        - slow-lun103
```

If the default StorageClass is not set explicitly, the first (alphabetically) StorageClass created by the module is used.

## CSI driver modes

Storage subsystem behavior is controlled by [`storageClass.compatibilityFlag`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-compatibilityflag):

| Value | Driver | Disk type | Online resize | Volume snapshots |
|-------|--------|-----------|---------------|------------------|
| not set (default) | `csi.vsphere.vmware.com` | CNS | Yes (vSphere 7.0U2+) | Yes |
| `Legacy` | `vsphere.csi.vmware.com` | FCD (First Class Disk) | No | No |
| `Migration` | both drivers simultaneously | CNS + FCD | Yes for CNS | Yes for CNS |

The `Migration` mode is intended for transitioning from the legacy FCD driver to CNS. After migrating all PVCs, clear `compatibilityFlag` (or remove the parameter) to disable the legacy driver.

{% alert level="warning" %}
Before migrating PVCs from FCD to CNS, ensure VM templates use hardware version 15 or later. See the [module documentation](/modules/cloud-provider-vsphere/configuration.html#csi) for details.
{% endalert %}

## Resizing a PVC

DKP supports online PersistentVolume resize in CNS mode (vSphere 7.0U2+). Due to [specifics](https://github.com/kubernetes-csi/external-resizer/issues/44) of the volume-resizer and vSphere API, additional steps are required after resizing a PVC:

1. Run `d8 k cordon <node_name>` on the node hosting the Pod using the volume.
1. Delete the Pod using the PVC.
1. Wait for the resize operation to complete:
   - Ensure the PVC no longer has the `Resizing` condition.
   - The `FileSystemResizePending` status can be ignored.
1. Run `d8 k uncordon <node_name>`.

## Volume snapshots

When the [`snapshot-controller`](/modules/snapshot-controller/) module is enabled, DKP automatically creates a `VolumeSnapshotClass` named `vsphere` for the CNS CSI driver. Snapshots are supported only in the standard mode (not in `Legacy`).

Example:

```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: my-snapshot
spec:
  volumeSnapshotClassName: vsphere
  source:
    persistentVolumeClaimName: my-pvc
```

## Datastore configuration for PVCs

For dynamic PersistentVolume provisioning to work correctly, a Datastore must be available on **every** ESXi host in the zone (shared datastore).

Assign region and zone tags to Datastore objects. You can do this via vSphere Client — see [Datastore configuration](authorization.html#configuring-datastore-in-vsphere-client), or via `govc`:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName2>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```

{% alert level="warning" %}
All Clusters within the same zone must have access to all Datastores tagged with that zone. For the region/zone model, see [Connection and authorization](authorization.html#creating-tags-and-tag-categories-in-vsphere-client).
{% endalert %}

## Load balancing

Options for organizing incoming traffic load balancing in a vSphere cluster:

### External load balancer

If your infrastructure already has an external load balancer (for example, hardware or NSX-T in reverse proxy mode), you can route traffic directly to the cluster frontend nodes.

### MetalLB (BGP)

For fault-tolerant in-cluster load balancing, use MetalLB in BGP mode:

- Frontend nodes receive two network interfaces.
- A dedicated VLAN is required for BGP traffic.
- The network must provide DHCP and internet access.
- BGP router IP addresses and ASNs must be specified.
- An IP address pool to announce must be defined.

{% alert level="info" %}
Ensure connectivity between BGP routers and frontend nodes in the dedicated VLAN.
{% endalert %}

### NSX-T Load Balancer (via cloud-controller-manager)

The `cloud-provider-vsphere` module supports creating `LoadBalancer` services via NSX-T integration. Configure the [`nsxt`](/modules/cloud-provider-vsphere/configuration.html#parameters-nsxt) section in `ModuleConfig`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-vsphere
spec:
  version: 2
  settings:
    nsxt:
      defaultIpPoolName: pool1
      tier1GatewayPath: /infra/tier-1s/gateway1
      host: nsx-manager.example.com
      user: admin
      password: "<PASSWORD>"
      insecureFlag: true
```

After configuration, `LoadBalancer` services receive an external IP from the NSX-T pool. To use alternative profiles and IP pools, set the `loadbalancer.vmware.io/class` annotation on the Service — see the [module documentation](/modules/cloud-provider-vsphere/configuration.html#parameters-nsxt).
