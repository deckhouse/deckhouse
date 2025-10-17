---
title: Connection and authorization
permalink: en/admin/integrations/virtualization/vsphere/authorization.html
---

## Requirements

For the proper operation of the Deckhouse Kubernetes Platform with VMware vSphere, the following are required:

- Access to vCenter;
- A user with the necessary set of permissions;
- Created tags and tag categories in vSphere;
- Networks with DHCP and internet access;
- Available shared datastores on all ESXi hosts.

* vSphere version: `v7.0U2` ([required](https://github.com/kubernetes-sigs/vsphere-csi-driver/blob/v2.3.0/docs/book/features/volume_expansion.md#vsphere-csi-driver---volume-expansion) for the `Online volume expansion` feature to work).
* vCenter: must be accessible from within the cluster from the master nodes.
* A created Datacenter containing:
  1. Virtual Machine template.
     * The VM image must use `Virtual machines with hardware version 15 or later` (required for online resize to work).
     * The following packages must be installed: `open-vm-tools`, `cloud-init`, and [`cloud-init-vmware-guestinfo`](https://github.com/vmware-archive/cloud-init-vmware-guestinfo#installation) (if using `cloud-init` version lower than 21.3).
  2. Network.
     * Must be available on all ESXi hosts where VMs will be created.
  3. Datastore (one or more).
     * Must be connected to all ESXi hosts where VMs will be created.
     * **Required**: assign a tag from the tag categories specified in [zoneTagCategory](#parameters-zonetagcategory) (default: `k8s-zone`). This tag will designate a **zone**. All clusters in a specific zone must have access to all datastores with the same zone tag.
  4. Cluster.
     * Contains the ESXi hosts to be used.
     * **Required**: assign a tag from the tag categories specified in [zoneTagCategory](#parameters-zonetagcategory) (default: `k8s-zone`). This tag will designate a **zone**.
  5. Folder for created VMs.
     * Optional (root VM folder is used by default).
  6. Role.
     * Must include the required [set](#list-of-required-privileges) of permissions.
  7. User.
     * Assigned the role from item 6.
* The created Datacenter **must** be assigned a tag from the tag category specified in [regionTagCategory](#parameters-regiontagcategory) (default: `k8s-region`). This tag will designate a **region**.

### Preparing the virtual machine image

To create a VM template (`Template`), it is recommended to use a ready-made cloud image/OVA file provided by the OS vendor:

* [**Ubuntu**](https://cloud-images.ubuntu.com/)
* [**Debian**](https://cloud.debian.org/images/cloud/)
* [**CentOS**](https://cloud.centos.org/)
* [**Rocky Linux**](https://rockylinux.org/alternative-images/) (section *Generic Cloud / OpenStack*)

{% alert %}
If you plan to use a domestic OS distribution, contact the OS vendor to obtain the image/OVA file.
{% endalert %}

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

### VM image requirements

DKP uses `cloud-init` to configure the VM after it starts. The following packages must be installed in the image:

* `open-vm-tools`
* `cloud-init`
* [`cloud-init-vmware-guestinfo`](https://github.com/vmware-archive/cloud-init-vmware-guestinfo#installation) (if using `cloud-init` version lower than 21.3)

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

Also, after the VM is started, the following services related to these packages must be running:

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

To add an SSH key, the `default_user` parameter must be specified in the `/etc/cloud/cloud.cfg` file.

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

{% alert %}
DKP creates VM disks of type `eagerZeroedThick`, but the type of disks of created VMs may be changed without notification according to the `VM Storage Policy` settings in vSphere.  
For more details, see the [documentation](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-single-host-management-vmware-host-client-8-0/virtual-machine-management-with-the-vsphere-host-client-vSphereSingleHostManagementVMwareHostClient/configuring-virtual-machines-in-the-vsphere-host-client-vSphereSingleHostManagementVMwareHostClient/virtual-disk-configuration-vSphereSingleHostManagementVMwareHostClient/about-virtual-disk-provisioning-policies-vSphereSingleHostManagementVMwareHostClient.html).
{% endalert %}

{% alert %}
DKP uses the `ens192` interface as the default interface for VMs in vSphere. Therefore, when using static IP addresses in [`mainNetwork`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-mainnetwork), you must create an interface named `ens192` in the OS image as the default interface.
{% endalert %}

## Installing govc

The [`govc`](https://github.com/vmware/govmomi/tree/main/govc) CLI tool is used for environment configuration.  
After installation, set the following environment variables:

```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<username>@vsphere.local
export GOVC_PASSWORD=<password>
export GOVC_INSECURE=1
```

## Tag and category configuration

vSphere does not have built-in concepts of regions and zones — instead, tags are used.

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

## Datastore configuration

For PersistentVolume to work correctly, the datastore must be accessible on all ESXi hosts.

Assign tags:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName2>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```

## Creating and assigning a role

Create a role with the necessary permissions:

```shell
govc role.create deckhouse \
  Cns.Searchable Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement \
  Global.GlobalTag Global.SystemTag Network.Assign StorageProfile.View \
  $(govc role.ls Admin | grep -F -e 'Folder.' -e 'InventoryService.' -e 'Resource.' -e 'VirtualMachine.')
```

Assign the role to a user:

```shell
govc permissions.set -principal <username>@vsphere.local -role deckhouse /
```

{% alert level="info" %}
For more detailed permission configuration, refer to the [official documentation](https://vmware.github.io/govmomi/).
{% endalert %}
