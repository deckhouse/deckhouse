---
title: Connection and authorization
permalink: en/admin/integrations/virtualization/vsphere/vsphere-authorization.html
---

## Requirements

To ensure proper operation of Deckhouse Kubernetes Platform with VMware vSphere, you need:

- Access to vCenter.
- A user with the necessary permissions.
- Created tags and tag categories in vSphere.
- Networks with DHCP and internet access.
- Shared datastores available on all ESXi hosts.

## Installing govc

The environment is configured using the CLI tool [`govc`](https://github.com/vmware/govmomi/tree/main/govc).
After installing, set the following environment variables:

```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<username>@vsphere.local
export GOVC_PASSWORD=<password>
export GOVC_INSECURE=1
```

## Configuring tags and categories

vSphere does not have built-in concepts of region or zone. Instead, it uses tags.

Create tag categories:

```shell
govc tags.category.create -d "Kubernetes Region" k8s-region
govc tags.category.create -d "Kubernetes Zone" k8s-zone
```

Create tags:

```shell
govc tags.create -d "Kubernetes Region" -c k8s-region test-region
govc tags.create -d "Kubernetes Zone Test 1" -c k8s-zone test-zone-1
govc tags.create -d "Kubernetes Zone Test 2" -c k8s-zone test-zone-2
```

Assign tags:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/host/<ClusterName1>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/host/<ClusterName2>
```

## Configuring datastore

For PersistentVolume to work correctly, the datastore must be accessible from all ESXi hosts.

Assign tags:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName2>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```

## Creating and assigning a role

Create a role with the required permissions:

```shell
govc role.create deckhouse \
  Cns.Searchable Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement \
  Global.GlobalTag Global.SystemTag Network.Assign StorageProfile.View \
  $(govc role.ls Admin | grep -F -e 'Folder.' -e 'InventoryService.' -e 'Resource.' -e 'VirtualMachine.')
```

Assign the role to the user:

```shell
govc permissions.set -principal <username>@vsphere.local -role deckhouse /
```

{% alert level="info" %}
For detailed permission configuration, refer to the [official documentation](https://vmware.github.io/govmomi/).
{% endalert %}
