---
title: "Cloud provider — VMware vSphere: Preparing environment"
description: "Configuring VMware vSphere for Deckhouse cloud provider operation."
---

<!-- AUTHOR! Don't forget to update getting started if necessary -->

## List of required vSphere resources

* **User** with required set of [permissions](#creating-and-assigning-a-role).
* **Network** with DHCP server and access to the Internet
* **Datacenter** with a tag in [`k8s-region`](#creating-tags-and-tag-categories) category.
* **Cluster** with a tag in [`k8s-zone`](#creating-tags-and-tag-categories) category.
* **Datastore** with required [tags](#datastore-configuration).
* **Template** — [prepared](#preparing-a-virtual-machine-image) VM image.

## vSphere configuration

### Installing govc

You'll need the vSphere CLI — [govc](https://github.com/vmware/govmomi/tree/master/govc#installation) — to proceed with the rest of the guide.

After the installation is complete, set the environment variables required to work with vCenter:

```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<username>@vsphere.local
export GOVC_PASSWORD=<password>
export GOVC_INSECURE=1
```

### Creating tags and tag categories

Instead of "regions" and "zones", VMware vSphere provides `Datacenter` and `Cluster` objects. We will use tags to match them with "regions"/"zones". These tags fall into two categories: one for "regions" tags and the other for "zones" tags.

Create a tag category using the following commands:

```shell
govc tags.category.create -d "Kubernetes Region" k8s-region
govc tags.category.create -d "Kubernetes Zone" k8s-zone
```

Create tags in each category. If you intend to use multiple "zones" (`Cluster`), create a tag for each one of them:

```shell
govc tags.create -d "Kubernetes Region" -c k8s-region test-region
govc tags.create -d "Kubernetes Zone Test 1" -c k8s-zone test-zone-1
govc tags.create -d "Kubernetes Zone Test 2" -c k8s-zone test-zone-2
```

Attach the "region" tag to `Datacenter`:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>
```

Attach "zone" tags to the `Cluster` objects:

```shell
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/host/<ClusterName1>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/host/<ClusterName2>
```

#### Datastore configuration

{% alert level="warning" %}
For dynamic `PersistentVolume` provisioning, a `Datastore` must be available on **each** ESXi host (shared datastore).
{% endalert %}

Assign the "region" and "zone" tags to the `Datastore` objects to automatically create a `StorageClass` in the Kubernetes cluster:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName2>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```

### Creating and assigning a role

{% alert %}
We've intentionally skipped User creation since there are many ways to authenticate a user in the vSphere.

This all-encompassing Role should be enough for all Deckhouse components. For a detailed list of privileges, refer to the [documentation](/modules/cloud-provider-vsphere/configuration.html#list-of-required-privileges). If you need a more granular Role, please contact your Deckhouse support.
{% endalert %}

Create a role with the corresponding permissions:

```shell
govc role.create deckhouse \
   Cns.Searchable Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement \
   Global.GlobalTag Global.SystemTag Network.Assign StorageProfile.View \
   $(govc role.ls Admin | grep -F -e 'Folder.' -e 'InventoryService.' -e 'Resource.' -e 'VirtualMachine.')
```

Assign the role to a user on the `vCenter` object:

```shell
govc permissions.set -principal <username>@vsphere.local -role deckhouse /
```

### Preparing a virtual machine image

It is recommended to use a pre-built cloud image/OVA file provided by the OS vendor to create a `Template`:

* [**Ubuntu**](https://cloud-images.ubuntu.com/)
* [**Debian**](https://cloud.debian.org/images/cloud/)
* [**CentOS**](https://cloud.centos.org/)
* [**Rocky Linux**](https://rockylinux.org/alternative-images/) (*Generic Cloud / OpenStack* section)

#### Virtual machine image requirements

Deckhouse uses `cloud-init` to configure a virtual machine after startup. To do this, the following packages must be installed in the image:

* `open-vm-tools`
* `cloud-init`
* [`cloud-init-vmware-guestinfo`](https://github.com/vmware-archive/cloud-init-vmware-guestinfo#installation) (if the `cloud-init` version lower than 21.3 is used)

Preparing the image for `cloud-init` on vSphere:

1. Install the required packages:

   ```shell
   sudo apt-get update
   sudo apt-get install -y open-vm-tools cloud-init # For cloud-init versions below 21.3, VMware GuestInfo support is required.
   ```

1. Verify that the `disable_vmware_customization: false` parameter is set in `/etc/cloud/cloud.cfg`.

1. Add the VMware GuestInfo datasource — create `/etc/cloud/cloud.cfg.d/99-DataSourceVMwareGuestInfo.cfg`:

   ```yaml
   datasource:
     VMware:
       vmware_cust_file_max_wait: 10
   ```

1. Before creating the VM template, reset identifiers and the `cloud-init` state:

   ```shell
   truncate -s 0 /etc/machine-id rm /var/lib/dbus/machine-id ln -s /etc/machine-id /var/lib/dbus/machine-id
   ```

1. Clear `cloud-init` event logs:

   ```shell
   cloud-init clean --logs --seed
   ```

Also, after the virtual machine is started, the following services associated with these packages must be started:

* `cloud-config.service`
* `cloud-final.service`
* `cloud-init.service`

You can start the services using the command:

```shell
systemctl is-enabled cloud-config.service cloud-init.service cloud-final.service
```

Example output:

```console
enabled
enabled
enabled
```

To add SSH keys to user's authorized keys, the `default_user` parameter must be specified in the `/etc/cloud/cloud.cfg` file.

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

{% alert level="warning" %}
Deckhouse creates virtual machine disks of the `eagerZeroedThick` type, however, the disk type of the created VMs will be changed without any notice to match the `VM Storage Policy` as configured in vSphere.
You can read more in the [documentation](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-single-host-management-vmware-host-client-8-0/virtual-machine-management-with-the-vsphere-host-client-vSphereSingleHostManagementVMwareHostClient/configuring-virtual-machines-in-the-vsphere-host-client-vSphereSingleHostManagementVMwareHostClient/virtual-disk-configuration-vSphereSingleHostManagementVMwareHostClient/about-virtual-disk-provisioning-policies-vSphereSingleHostManagementVMwareHostClient.html).
{% endalert %}

{% alert %}
Deckhouse uses the `ens192` interface as the default interface for virtual machines in vSphere. Therefore, when using static IP addresses in `mainNetwork`, you must create an interface named `ens192` in the OS image as the default interface.
{% endalert %}

## Infrastructure

### Networking

A VLAN with DHCP and Internet access is required for the running cluster:

* If the VLAN is public (public addresses), then you have to create a second network to deploy cluster nodes (DHCP is not needed in this network).
* If the VLAN is private (private addresses), then this network can be used for cluster nodes.

### Inbound traffic

* You can use an internal load balancer (if present) and direct traffic directly to the front nodes of the cluster.
* If there is no load balancer, you can use MetalLB in BGP mode to organize fault-tolerant load balancers (recommended). In this case, front nodes of the cluster will have two interfaces. For this, you will need:
  * A dedicated VLAN for traffic exchange between BGP routers and MetalLB. This VLAN must have DHCP and Internet access.
  * IP addresses of BGP routers.
  * ASN — the AS number on the BGP router.
  * ASN — the AS number in the cluster.
  * A range to announce addresses from.

### Using the data store

Various types of storage can be used in the cluster; for the minimum configuration, you will need:
* Datastore for provisioning PersistentVolumes to the Kubernetes cluster.
* Datastore for provisioning root disks for the VMs (it can be the same Datastore as for PersistentVolume).
