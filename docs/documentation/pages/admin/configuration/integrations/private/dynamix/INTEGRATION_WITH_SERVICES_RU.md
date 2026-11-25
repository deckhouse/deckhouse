---
title: Управление узлами в Basis Dynamix
permalink: ru/admin/integrations/private/dynamix/node-management.html
lang: ru
---

## Настройка параметров узлов

Пример описания DynamixInstanceClass:

```yaml
apiVersion: deckhouse.io/v1
kind: DynamixInstanceClass
metadata:
  name: frontend
spec:
  numCPUs: 4
  memory: 8192
  rootDiskSizeGb: 40
  imageName: alt-p10-cloud-x86_64.img
  storagePolicy: storage_policy01
  externalNetwork: extnet_vlan_1700
```

 На него ссылается [параметр cloudInstances.classReference](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-classreference) NodeGroup.
