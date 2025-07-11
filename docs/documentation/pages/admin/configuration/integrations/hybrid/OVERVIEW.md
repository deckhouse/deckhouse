---
title: Hybrid integrations
permalink: en/admin/integrations/hybrid/overview.html
---

Deckhouse Kubernetes Platform (DKP) can use cloud provider resources to expand the capacity of static clusters.
Currently, integration is supported with [OpenStack](../public/openstack/сonnection-and-authorization.html) and [vSphere](../public/vsphere/vsphere-authorization.html)-based clouds.

A hybrid cluster is a Kubernetes cluster that combines bare-metal nodes with nodes running on vSphere or OpenStack.
To create such a cluster, an L2 network must be available between all nodes.

## Hybrid cluster with vSphere

Follow these steps:

1. Remove `flannel` from the `kube-system` namespace:

   ```shell
   kubectl -n kube-system delete ds flannel-ds
   ```

1. Configure the integration and set the required parameters.

{% alert level="warning" %}
`Cloud-controller-manager` synchronizes state between vSphere and Kubernetes,
removing nodes from Kubernetes that are not present in vSphere.
In a hybrid cluster, this behavior is not always desirable.
Therefore, any Kubernetes node not launched with the `--cloud-provider=external` flag will be automatically ignored.
DKP automatically sets `static://` in the `.spec.providerID` field of such nodes, which `cloud-controller-manager` then ignores.
{% endalert %}

## Hybrid cluster with OpenStack

Follow these steps:

1. Remove `flannel` from the `kube-system` namespace:

   ```shell
   kubectl -n kube-system delete ds flannel-ds
   ```

1. Configure the integration and set the required parameters.
1. Create one or more [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass) custom resources.
1. Create one or more [NodeGroup](/modules/node-manager/cr.html#nodegroup) resources to manage the number and provisioning of cloud-based VMs.

{% alert level="warning" %}
`Cloud-controller-manager` synchronizes state between OpenStack and Kubernetes,
removing nodes from Kubernetes that are not present in OpenStack.
In a hybrid cluster, this behavior is not always desirable.
Therefore, any Kubernetes node not launched with the `--cloud-provider=external` flag will be automatically ignored.
DKP automatically sets `static://` in the `.spec.providerID` field of such nodes, which `cloud-controller-manager` then ignores.
{% endalert %}

## Storage integration

If you need PersistentVolumes on nodes provisioned from OpenStack,
create a StorageClass with the target OpenStack volume type.
You can list the available types using the following command:

```shell
openstack volume type list
```

Example for `ceph-ssd` volume type:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ceph-ssd
provisioner: csi-cinderplugin # Leave this as shown here.
parameters:
  type: ceph-ssd
volumeBindingMode: WaitForFirstConsumer
```
