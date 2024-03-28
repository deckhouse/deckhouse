---
title: "Cloud provider — VMware Cloud Director: Примеры"
---

Ниже представлен пример конфигурации cloud-провайдера vCloud Director.

## Пример custom resource `VCDInstanceClass`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: VCDInstanceClass
metadata:
  name: test
spec:
  rootDiskSizeGb: 90
  sizingPolicy: payg-4-8
  storageProfile: SSD-dc1-pub1-cl1
  template: user-123456
```
