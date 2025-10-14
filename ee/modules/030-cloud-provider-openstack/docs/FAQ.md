---
title: "Cloud provider — OpenStack: FAQ"
---

## How do I set up LoadBalancer?

> **Note!** Load Balancer must support Proxy Protocol to determine the client IP correctly.

### An example of IngressNginxController

Below is a simple example of the `IngressNginxController' configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancerWithProxyProtocol
  loadBalancerWithProxyProtocol:
    annotations:
      loadbalancer.openstack.org/proxy-protocol: "true"
      loadbalancer.openstack.org/timeout-member-connect: "2000"
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    operator: Equal
    value: frontend
```

## How do I set up security policies on cluster nodes?

There may be many reasons why you may need to restrict or expand incoming/outgoing traffic on cluster VMs in OpenStack:

* Allow VMs on a different subnet to connect to cluster nodes.
* Allow connecting to the ports of the static node so that the application can work.
* Restrict access to external resources or other VMs in the cloud for security reasons.

For all this, additional security groups should be used. You can only use security groups that are created in the cloud tentatively.

### Enabling additional security groups on static and master nodes

This parameter can be set either in an existing cluster or when creating one. In both cases, additional security groups are declared in the `OpenStackClusterConfiguration`:

* for master nodes, in the `additionalSecurityGroups` of the `masterNodeGroup` section;
* for static nodes, in the `additionalSecurityGroups` field of the `nodeGroups` subsection that corresponds to the target nodeGroup.

The `additionalSecurityGroups` field contains an array of strings with security group names.

### Enabling additional security groups on ephemeral nodes

You have to set the `additionalSecurityGroups` parameter for all OpenStackInstanceClasses in the cluster that require additional security groups. See the [parameters of the cloud-provider-openstack](../../modules/cloud-provider-openstack/configuration.html) module.

## How do I create a hybrid cluster?

A hybrid cluster combines bare metal and OpenStack nodes. To create such a cluster, you need an L2 network between all nodes of the cluster.

{% alert level="info" %}
The Deckhouse Kubernetes Platform allows to set a prefix for the names of CloudEphemeral nodes added to a hybrid cluster with Static master nodes.
To do this, use the [`instancePrefix`](../node-manager/configuration.html#parameters-instanceprefix) parameter of the `node-manager` module. The prefix specified in the parameter will be added to the name of all CloudEphemeral nodes added to the cluster. It is not possible to set a prefix for a specific NodeGroup.
{% endalert %}

To set up a hybrid cluster, follow these steps:

1. Delete flannel from kube-system: `d8 k -n kube-system delete ds flannel-ds`.
2. Enable and [configure](configuration.html#parameters) the module.
3. Create one or more [OpenStackInstanceClass](cr.html#openstackinstanceclass) custom resources.
4. Create one or more [NodeManager](../../modules/node-manager/cr.html#nodegroup) custom resources for specifying the number of machines and managing the provisioning process in the cloud.

> **Caution!** Cloud-controller-manager synchronizes OpenStack and Kubernetes states by deleting Kubernetes nodes that are not in OpenStack. In a hybrid cluster, such behavior does not always make sense. That is why cloud-controller-manager automatically skips Kubernetes nodes that do not have the `--cloud-provider=external` parameter (Deckhouse inserts `static://` into nodes in `.spec.providerID`, and cloud-controller-manager ignores them).

### Attaching storage devices to instances in a hybrid cluster

To use PersistentVolumes on OpenStack nodes, you must create StorageClass with the appropriate OpenStack volume type. The `openstack volume type list` command lists all available types.

Here is the example config for the `ceph-ssd` volume type:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ceph-ssd
provisioner: csi-cinderplugin # have to be like this
parameters:
  type: ceph-ssd
volumeBindingMode: WaitForFirstConsumer
```

## How do I create an image in OpenStack?

1. Download the latest stable Ubuntu 18.04 image:

   ```shell
   curl -L https://cloud-images.ubuntu.com/bionic/current/bionic-server-cloudimg-amd64.img --output ~/ubuntu-18-04-cloud-amd64
   ```

2. Prepare an OpenStack RC (openrc) file containing credentials for accessing the OpenStack API:

   > The interface for getting an openrc file may differ depending on the OpenStack provider. If the provider has a standard interface for OpenStack, you can download the openrc file using the following [instruction](https://docs.openstack.org/ocata/admin-guide/common/cli-set-environment-variables-using-openstack-rc.html#download-and-source-the-openstack-rc-file).

3. Otherwise, install the OpenStack client using this [instruction](https://docs.openstack.org/newton/user-guide/common/cli-install-openstack-command-line-clients.html).

   Also, you can run the container and mount an openrc file and a downloaded Ubuntu image in it:

   ```shell
   docker run -ti --rm -v ~/ubuntu-18-04-cloud-amd64:/ubuntu-18-04-cloud-amd64 -v ~/.openrc:/openrc jmcvea/openstack-client
   ```

4. Initialize the environment variables from the openrc file:

   ```shell
   source /openrc
   ```

5. Get a list of available disk types:

   ```shell
   / # openstack volume type list
   +--------------------------------------+---------------+-----------+
   | ID                                   | Name          | Is Public |
   +--------------------------------------+---------------+-----------+
   | 8d39c9db-0293-48c0-8d44-015a2f6788ff | ko1-high-iops | True      |
   | bf800b7c-9ae0-4cda-b9c5-fae283b3e9fd | dp1-high-iops | True      |
   | 74101409-a462-4f03-872a-7de727a178b8 | ko1-ssd       | True      |
   | eadd8860-f5a4-45e1-ae27-8c58094257e0 | dp1-ssd       | True      |
   | 48372c05-c842-4f6e-89ca-09af3868b2c4 | ssd           | True      |
   | a75c3502-4de6-4876-a457-a6c4594c067a | ms1           | True      |
   | ebf5922e-42af-4f97-8f23-716340290de2 | dp1           | True      |
   | a6e853c1-78ad-4c18-93f9-2bba317a1d13 | ceph          | True      |
   +--------------------------------------+---------------+-----------+
   ```

6. Create an image, pass the disk format to use (if OpenStack does not support local disks or these disks don't fit):

   ```shell
   openstack image create --private --disk-format qcow2 --container-format bare \
     --file /ubuntu-18-04-cloud-amd64 --property cinder_img_volume_type=dp1-high-iops ubuntu-18-04-cloud-amd64
   ```

7. Check that the image was created successfully:

   ```text
   / # openstack image show ubuntu-18-04-cloud-amd64
   +------------------+-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
   | Field            | Value                                                                                                                                                                                                                                                                                     |
   +------------------+-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
   | checksum         | 3443a1fd810f4af9593d56e0e144d07d                                                                                                                                                                                                                                                          |
   | container_format | bare                                                                                                                                                                                                                                                                                      |
   | created_at       | 2020-01-10T07:23:48Z                                                                                                                                                                                                                                                                      |
   | disk_format      | qcow2                                                                                                                                                                                                                                                                                     |
   | file             | /v2/images/01998f40-57cc-4ce3-9642-c8654a6d14fc/file                                                                                                                                                                                                                                      |
   | id               | 01998f40-57cc-4ce3-9642-c8654a6d14fc                                                                                                                                                                                                                                                      |
   | min_disk         | 0                                                                                                                                                                                                                                                                                         |
   | min_ram          | 0                                                                                                                                                                                                                                                                                         |
   | name             | ubuntu-18-04-cloud-amd64                                                                                                                                                                                                                                                                  |
   | owner            | bbf506e3ece54e21b2acf1bf9db4f62c                                                                                                                                                                                                                                                          |
   | properties       | cinder_img_volume_type='dp1-high-iops', direct_url='rbd://b0e441fc-c317-4acf-a606-cf74683978d2/images/01998f40-57cc-4ce3-9642-c8654a6d14fc/snap', locations='[{u'url': u'rbd://b0e441fc-c317-4acf-a606-cf74683978d2/images/01998f40-57cc-4ce3-9642-c8654a6d14fc/snap', u'metadata': {}}]' |
   | protected        | False                                                                                                                                                                                                                                                                                     |
   | schema           | /v2/schemas/image                                                                                                                                                                                                                                                                         |
   | size             | 343277568                                                                                                                                                                                                                                                                                 |
   | status           | active                                                                                                                                                                                                                                                                                    |
   | tags             |                                                                                                                                                                                                                                                                                           |
   | updated_at       | 2020-05-01T17:18:34Z                                                                                                                                                                                                                                                                      |
   | virtual_size     | None                                                                                                                                                                                                                                                                                      |
   | visibility       | private                                                                                                                                                                                                                                                                                   |
   +------------------+-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
   ```

## How to check whether the provider supports SecurityGroups?

Run the following command: `openstack security group list`. If there are no errors in the output, then [Security Groups](https://docs.openstack.org/nova/pike/admin/security-groups.html) are supported.

## How to set up online disk resize

The OpenStack API states that the resize is completed successfully. However, Nova does not get any information about the resize from Cinder. As a result, the size of the disk in the guest OS remains the same.

To get rid of this problem, you need to insert the Nova API access parameters into the `cinder.conf` file, e.g., as follows:

{% raw %}

```ini
[nova]
interface = admin
insecure = {{ keystone_service_internaluri_insecure | bool }}
auth_type = {{ cinder_keystone_auth_plugin }}
auth_url = {{ keystone_service_internaluri }}/v3
password = {{ nova_service_password }}
project_domain_id = default
project_name = service
region_name = {{ nova_service_region }}
user_domain_id = default
username = {{ nova_service_user_name }}
```

{% endraw %}

[Source...](https://bugs.launchpad.net/openstack-ansible/+bug/1902914)

## How to use `rootDiskSize` and when it is preferred?

### Disks in OpenStack

The node disk can be local or network. A local disk in OpenStack, is an ephemeral disk, and a network disk is a persistent disk (cinder storage). Nodes with local disks cannot migrate between hypervisors.

* A network disk is preferred for the master node so that the node can migrate between hypervisors.
* A local disk is preffered for the ephemeral node to save on cost. Not all cloud providers support the use of local disks. If local disks are not supported, you have to use network disks for ephemeral nodes.

| Local disk (ephemeral)        | Network disk (persistent)                    |
| ----------------------------- | -------------------------------------------- |
| Cheaper                       | More expensive                               |
| Suitable for ephemeral nodes  | Suitable for master nodes                    |

### The `rootDiskSize` parameter

The `OpenStackInstanceClass` has a `rootDiskSize` parameter, and OpenStack flavor has a disk size parameter. Which disk will be ordered depending on the combination of parameters is shown in the table:

|                                     | flavor disk size = 0                 | flavor disk size > 0                              |
| ----------------------------------- | ------------------------------------ | ------------------------------------------------- |
| **`rootDiskSize` is not specified** | ❗️*You need to set the size*. Without specifying the size, there will be an error creating a VM. | Local disk with size according to the flavor    |
| **`rootDiskSize` is specified**     | Network disk with the `rootDiskSize` size                                         | ❗ Network disk (rootDiskSize) and local disk (according to the flavor). Avoid using this option, as the cloud provider will charge for both disks. |

> Please note, that to create a node with the `CloudEphemeral` type in a zone other than zone A, you must first create a flavor with a disk of the required size. The [rootDiskSize](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-rootdisksize) parameter does not need to be specified.

#### Network disk is recommended for master nodes and bastion host

* Use flavor with a zero disk size.
* Set the `rootDiskSize` in the `OpenStackInstanceClass`.
* Check the disk type. The disk type will be taken from the OS image if it is [set](#how-to-override-a-default-volume-type-of-cloud-provider). If it is not set, the disk type will be taken from [volumeTypeMap](cluster_configuration.html#openstackclusterconfiguration-masternodegroup-volumetypemap).

#### Local disk is recommended for ephemeral nodes

* Use flavor with the specified disk size.
* Do not use the `rootDiskSize` parameter in the `OpenStackInstanceClass`.
* Check the disk type. The disk type will be taken from the OS image if it is [set](#how-to-override-a-default-volume-type-of-cloud-provider). If it is not set, the default disk type of the cloud provider will be used.

### How do I check the disk volume in a flavor?

```shell
# openstack flavor show m1.medium-50g -c disk
+-------+-------+
| Field | Value |
+-------+-------+
| disk  | 50    |
+-------+-------+
```

## How to override a default volume type of cloud provider?

If there are several types of disks in a *cloud provider*, you can set a default disk type for the image in order to select a specific VM's disk type. To do this, specify the name of a disk type in the image metadata.

Also, you may need to create a custom OpenStack image; the ["How do I create an image in OpenStack"](#how-do-i-create-an-image-in-openstack) section describes how to do it

Example:

```shell
openstack volume type list
openstack image set ubuntu-18-04-cloud-amd64 --property cinder_img_volume_type=VOLUME_NAME
```

## OFFLINE disk resize

Some cloud providers may not support ONLINE disk resizing.
If you get the following error, then you need to reduce the number of StatefulSet replicas to 0, wait for disk resizing
and return the number of replicas that was before the start of the operation.

```text
Warning  VolumeResizeFailed     5s (x11 over 41s)  external-resizer cinder.csi.openstack.org                                   
resize volume "pvc-555555-ab66-4f8d-947c-296520bae4c1" by resizer "cinder.csi.openstack.org" failed: 
rpc error: code = Internal desc = Could not resize volume "bb5a275b-3f30-4916-9480-9efe4b6dfba5" to size 2: 
Expected HTTP response code [202] when accessing 
[POST https://public.infra.myfavourite-cloud-provider.ru:8776/v3/555555555555/volumes/bb5a275b-3f30-4916-9480-9efe4b6dfba5/action], but got 406 instead
{"computeFault": {"message": "Version 3.42 is not supported by the API. Minimum is 3.0 and maximum is 3.27.", "code": 406}}
```
