---
title: Integration with Huawei Cloud services
permalink: en/admin/integrations/private/huaweicloud/huawei-services.html
---

Deckhouse Kubernetes Platform leverages Huawei Cloud's capabilities to operate Kubernetes clusters.
The following features are supported:

- Resource management in Huawei Cloud via `cloud-controller-manager`.
- Disk provisioning using the CSI driver.
- Integration with the `node-manager` module, allowing the use of HuaweicloudInstanceClass in NodeGroup.

## Working with InstanceClass

The HuaweiCloudInstanceClass resource is used to define the parameters of virtual machines.
It is referenced by NodeGroup and CloudInstanceClass.

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
