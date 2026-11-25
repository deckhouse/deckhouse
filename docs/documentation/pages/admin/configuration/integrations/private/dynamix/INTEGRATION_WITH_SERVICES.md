---
title: Node management in Basis Dynamix
permalink: en/admin/integrations/private/dynamix/node-management.html
---

## Configuring node parameters

Example DynamixInstanceClass:

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

It is referenced by the NodeGroup's [`cloudInstances.classReference`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-classreference) parameter.
