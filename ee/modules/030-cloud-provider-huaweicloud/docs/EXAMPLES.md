---
title: "Cloud provider — HuaweiCloud: examples"
---

Below is an example configuration for a HuaweiCloud cloud provider.

## An example of the `HuaweiCloudInstanceClass` custom resource

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
