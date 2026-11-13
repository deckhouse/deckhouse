---
title: Integration with Huawei Cloud services
permalink: en/admin/integrations/private/huaweicloud/services.html
---

Deckhouse Platform leverages Huawei Cloud's capabilities to operate Kubernetes clusters.
The following features are supported:

- Resource management in Huawei Cloud via `cloud-controller-manager`.
- Disk provisioning using the [CSI driver](../../../../glossary.html#csi-driver).
- Integration with the `node-manager` module, allowing the use of [HuaweiCloudInstanceClass](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass) in [NodeGroup](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-classreference).

## Working with InstanceClass

The [HuaweiCloudInstanceClass](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass) resource is used to define the parameters of virtual machines.
It is referenced by [NodeGroup](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-classreference).

Example resource:

```yaml
apiVersion: deckhouse.io/v1
kind: HuaweiCloudInstanceClass
metadata:
  name: worker
spec:
  imageName: alt-p11
  flavorName: s7n.xlarge.2
  rootDiskSize: 50
  rootDiskType: SSD
```

## Replacing the OS image

The OS image of the nodes is set by the [`imageName`](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass-v1-spec-imagename) parameter of the HuaweiCloudInstanceClass resource for `CloudEphemeral` nodes and in the `instanceClass` section of the HuaweiCloudClusterConfiguration resource for `CloudPermanent` nodes and master nodes. To move the cluster nodes to a new image, follow the steps from ["How do I replace the OS image in a cloud cluster?"](../../../../faq.html#how-do-i-replace-the-os-image-in-a-cloud-cluster).
