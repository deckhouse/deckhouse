---
title: "The csi-vsphere module: Preparing environment"
description: "Tags, datastores, user accounts, and permissions in vSphere before enabling the csi-vsphere module."
---

This section describes how to prepare vCenter and vSphere for the `csi-vsphere` module.

## Required resources

The following resources are required for the module to function:

- User with the [required permissions](#create-and-assign-the-role).
- Network with DHCP and internet access.
- Datacenter tagged as [`k8s-region`](#create-tags-and-tag-categories).
- Cluster tagged as [`k8s-zone`](#create-tags-and-tag-categories).
- One or more Datastores with the appropriate [tags](#datastore-configuration).

## Installing govc

Use the vSphere CLI [govc](https://github.com/vmware/govmomi/tree/master/govc#installation).

Set environment variables for vCenter access:

```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<username>@vsphere.local
export GOVC_PASSWORD=<password>
export GOVC_INSECURE=1
```

## Create tags and tag categories

`csi-vsphere` maps Kubernetes topology to vSphere objects: a **region** is a vSphere Datacenter, and a **zone** is a vSphere Cluster. The relationship between these objects is defined using tags.

To link Cluster and Datacenter objects, follow these steps:

1. Create tag categories:

   ```shell
   govc tags.category.create -d "Kubernetes Region" k8s-region
   govc tags.category.create -d "Kubernetes Zone" k8s-zone
   ```

1. Create tags in each category. If you use several zones (Cluster), create one tag per cluster:

   ```shell
   govc tags.create -d "Kubernetes Region" -c k8s-region test-region
   govc tags.create -d "Kubernetes Zone Test 1" -c k8s-zone test-zone-1
   govc tags.create -d "Kubernetes Zone Test 2" -c k8s-zone test-zone-2
   ```

1. Attach the region tag to the Datacenter:

   ```shell
   govc tags.attach -c k8s-region test-region /<DatacenterName>
   ```

1. Attach zone tags to Cluster objects:

   ```shell
   govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/host/<ClusterName1>
   govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/host/<ClusterName2>
   ```

## Datastore configuration

{% alert level="info" %}
For dynamic PersistentVolume provisioning, each Datastore must be reachable from every ESXi host (shared datastore).
{% endalert %}

Attach region and zone tags to Datastore objects so the module can create StorageClass resources automatically:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName2>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```

## Create and assign the role

{% alert level="info" %}
Due to the variety of SSO providers connected to csi-vsphere, the steps for creating a user are not covered in this article.

The role to be created below includes all possible privileges for all DKP components. For a detailed list of privileges, refer to the [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/environment.html#list-of-required-privileges) documentation.
{% endalert %}

Create the role:

```shell
govc role.create deckhouse \
   Cns.Searchable Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement \
   Global.GlobalTag Global.SystemTag Network.Assign StorageProfile.View \
   $(govc role.ls Admin | grep -F -e 'Folder.' -e 'InventoryService.' -e 'Resource.' -e 'VirtualMachine.')
```

Grant the role on the `vCenter` root object:

```shell
govc permissions.set -principal <username>@vsphere.local -role deckhouse /
```
