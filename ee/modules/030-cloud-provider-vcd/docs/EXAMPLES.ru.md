---
title: "Cloud provider — VMware Cloud Director: Примеры"
---

Ниже представлен пример конфигурации [VCDInstanceClass](cr.html#vcdinstanceclass) для эфемерных узлов cloud-провайдера VMware Cloud Director.

## Пример конфигурации кастомного ресурса VCDInstanceClass

```yaml
apiVersion: deckhouse.io/v1
kind: VCDInstanceClass
metadata:
  name: test
spec:
  rootDiskSizeGb: 90
  sizingPolicy: payg-4-8
  storageProfile: SSD-dc1-pub1-cl1
  template: MyOrg/Linux/ubuntu2204-cloud-ova
```
