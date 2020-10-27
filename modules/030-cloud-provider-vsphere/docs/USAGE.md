---
title: "Сloud provider — VMware vSphere: Примеры конфигурации"
---

## Пример CR `VsphereInstanceClass`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: VsphereInstanceClass
metadata:
  name: test
spec:
  numCPUs: 2
  memory: 2048
  rootDiskSize: 20
  template: dev/golden_image
  network: k8s-msk-178
  datastore: lun-1201
```
