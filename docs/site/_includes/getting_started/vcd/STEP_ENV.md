{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

## List of required vSphere resources

* **User** with required set of [permissions](#creating-and-assigning-a-role).
* **Network** with DHCP server and access to the Internet
* **Datacenter** with a tag in [`k8s-region`](#creating-tags-and-tag-categories) category.
* **Cluster** with a tag in [`k8s-zone`](#creating-tags-and-tag-categories) category.
* **Datastore** with required [tags](#datastore-configuration).
* **Template** — the [prepared](#preparing-a-virtual-machine-image) VM image.

## vSphere configuration

### Installing govc

You'll need the vSphere CLI — [govc](https://github.com/vmware/govmomi/tree/master/govc#installation) — to proceed with the rest of the guide.

After the installation is complete, set the environment variables required to work with vCenter:

{% snippetcut %}
```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<username>@vsphere.local
export GOVC_PASSWORD=<password>
export GOVC_INSECURE=1
```
{% endsnippetcut %}

### Creating tags and tag categories

Instead of "regions" and "zones", VMware vSphere provides `Datacenter` and `Cluster` objects. We will use tags to match them with "regions"/"zones". These tags fall into two categories: one for "regions" tags and the other for "zones" tags.

Create a tag category using the following commands:

{% snippetcut %}
```shell
govc tags.category.create -d "Kubernetes Region" k8s-region
govc tags.category.create -d "Kubernetes Zone" k8s-zone
```
{% endsnippetcut %}

Create tags in each category. If you intend to use multiple "zones" (`Cluster`), create a tag for each one of them:

{% snippetcut %}
```shell
govc tags.create -d "Kubernetes Region" -c k8s-region test-region
govc tags.create -d "Kubernetes Zone Test 1" -c k8s-zone test-zone-1
govc tags.create -d "Kubernetes Zone Test 2" -c k8s-zone test-zone-2
```
{% endsnippetcut %}

Attach the "region" tag to `Datacenter`:

{% snippetcut %}
```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>
```
{% endsnippetcut %}

Attach "zone" tags to `Cluster` objects:

{% snippetcut %}
```shell
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/host/<ClusterName1>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/host/<ClusterName2>
```
{% endsnippetcut %}

#### Datastore configuration

{% alert level="warning" %}
For dynamic `PersistentVolume` provisioning, a `Datastore` must be available on **each** ESXi host (shared datastore).
{% endalert %}

Assign the "region" and "zone" tags to the `Datastore` objects to automatically create a `StorageClass` in the Kubernetes cluster:

{% snippetcut %}
```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```
{% endsnippetcut %}

### Creating and assigning a role

{% alert %}
We've intentionally skipped User creation since there are many ways to authenticate a user in the vSphere.

This all-encompassing Role should be enough for all Deckhouse components. For a detailed list of privileges, refer to the [documentation](/documentation/v1/modules/030-cloud-provider-vsphere/configuration.html#list-of-privileges-for-using-the-module). If you need a more granular Role, please contact your Deckhouse support.
{% endalert %}

Create a role with the corresponding permissions:

{% snippetcut %}
```shell
govc role.create deckhouse \
   Cns.Searchable Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement \
   Global.GlobalTag Global.SystemTag Network.Assign StorageProfile.View \
   $(govc role.ls Admin | grep -F -e 'Folder.' -e 'InventoryService.' -e 'Resource.' -e 'VirtualMachine.')
```
{% endsnippetcut %}

Assign the role to a user on the `vCenter` object:

{% snippetcut %}
```shell
govc permissions.set -principal <username>@vsphere.local -role deckhouse /
```
{% endsnippetcut %}

### Preparing a virtual machine image

It is recommended to use a pre-built cloud image/OVA file provided by the OS vendor to create a `Template`:

* [**Ubuntu**](https://cloud-images.ubuntu.com/)
* [**Debian**](https://cloud.debian.org/images/cloud/)
* [**CentOS**](https://cloud.centos.org/)
* [**Rocky Linux**](https://rockylinux.org/alternative-images/) (*Generic Cloud / OpenStack* section)

If you need to use your own image, please refer to the [documentation](/documentation/v1/modules/030-cloud-provider-vsphere/environment.html#virtual-machine-image-requirements).
