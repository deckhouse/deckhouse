---
title: "vSphere data storage"
permalink: en/admin/configuration/storage/external/vsphere.html
---

The `csi-vsphere` module is designed for provisioning disks in static clusters based on VMware vSphere, where it is not possible to use the [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/) module.

## System requirements

- All virtual machines in the cluster must be created using vSphere tools.
- The virtual machine name in vSphere must exactly match the node's hostname in the Deckhouse Kubernetes Platform cluster.
- The `disk.EnableUUID:TRUE` parameter must be enabled in the settings of each virtual machine. This parameter ensures the correct operation of the module with disk resources and allows DKP to identify the attached volumes.

## Enabling the module

To work with storage based on VMware vSphere, where it is not possible to use the `cloud-provider-vsphere` module, enable the `csi-vsphere` module. This will cause the following to occur on all cluster nodes:

- The CSI driver will be registered;
- The service pods for the `csi-vsphere` component will be launched.

To enable the module, run the command:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-vsphere
spec:
  enabled: true
  version: 1
  settings:
    # Required parameters.
    host: myhost
    password: myPaSsWd
    region: myreg
    regionTagCategory: myregtagcat
    username: myuname
    vmFolderPath: dev/test
    zoneTagCategory: myzonetagcat
    zones:
      - zonea
      - zoneb
EOF
```

Wait until the module reaches the `Ready` state. You can check the status by running the following command:

```shell
d8 k get module csi-vsphere -w
```

The output will display information about the `csi-vsphere` module:

```console
NAME         WEIGHT    STATE     SOURCE     STAGE   STATUS
csi-vsphere   910      Enabled   Embedded           Ready
```

## Environment preparation

### Required resources

* **User** with the necessary [permissions](#creating-and-assigning-roles).
* **Network** with DHCP and internet access.
* **Datacenter** with the corresponding tag [`k8s-region`](#creating-tags-and-tag-categories).
* **Cluster** with the corresponding tag [`k8s-zone`](#creating-tags-and-tag-categories).
* **Datastore** in any quantity with the corresponding [tags](#datastore-configuration).

### Installing govc

For further configuration of `csi-vsphere`, you will need the vSphere CLI — [govc](https://github.com/vmware/govmomi/tree/master/govc#installation).

After installation, set the environment variables for working with vCenter:

```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<username>@vsphere.local
export GOVC_PASSWORD=<password>
export GOVC_INSECURE=1
```

### Creating tags and tag categories

In `csi-vsphere`, there are no concepts of "region" and "zone". In `csi-vsphere`, the Datacenter is treated as the "region", and the Cluster is treated as the "zone". Tags are used to create this association.

Create tag categories using the following commands:

```shell
govc tags.category.create -d "Kubernetes Region" k8s-region
govc tags.category.create -d "Kubernetes Zone" k8s-zone
```

Create tags within each category. If you plan to use multiple "zones" (`Cluster`), create a tag for each one:

```shell
govc tags.create -d "Kubernetes Region" -c k8s-region test-region
govc tags.create -d "Kubernetes Zone Test 1" -c k8s-zone test-zone-1
govc tags.create -d "Kubernetes Zone Test 2" -c k8s-zone test-zone-2
```

Assign the "region" tag to the `Datacenter`:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>
```

Assign the "zone" tags to the `Cluster` objects:

```shell
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/host/<ClusterName1>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/host/<ClusterName2>
```

### Datastore configuration

{% alert level="warning" %}
For dynamic provisioning of `PersistentVolume`, it is required that the `Datastore` is available on **every** ESXi host (shared datastore).
{% endalert %}

To automatically create a StorageClass in the cluster, assign the previously created "region" and "zone" tags to the `Datastore` objects:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName2>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```

### Creating and assigning roles

{% alert %}
Due to the variety of SSO providers connected to `csi-vsphere`, the steps for creating a user are not covered in this article.

The role to be created below includes all possible privileges for all DKP components.
For a detailed list of privileges, refer to [the documentation](/modules/cloud-provider-vsphere/configuration.html#list-of-required-privileges).
{% endalert %}

Create the role with the necessary privileges:

```shell
govc role.create deckhouse \
   Cns.Searchable Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement \
   Global.GlobalTag Global.SystemTag Network.Assign StorageProfile.View \
   $(govc role.ls Admin | grep -F -e 'Folder.' -e 'InventoryService.' -e 'Resource.' -e 'VirtualMachine.')
```

Assign the role to the user on the `vCenter` object:

```shell
govc permissions.set -principal <username>@vsphere.local -role deckhouse /
```

## Creating StorageClass

The module automatically creates a StorageClass for each Datastore and DatastoreCluster from zones.

It also allows you to configure the name of the default StorageClass to be used in the cluster (parameter [default](../../../reference/api/global.html#parameters-defaultclusterstorageclass)) and filter out unnecessary StorageClasses (parameter [exclude](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-exclude)).
