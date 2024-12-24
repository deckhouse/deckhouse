---
title: "Cloud provider — OpenStack: примеры"
---

Ниже представлен пример конфигурации cloud-провайдера HuaweiCloud.

## Пример custom resource `HuaweiCloudInstanceClass`

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
