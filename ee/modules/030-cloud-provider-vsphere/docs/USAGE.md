---
title: "Сloud provider — VMware vSphere: usage"
---

## An example of the `VsphereInstanceClass` CR

```yaml
apiVersion: deckhouse.io/v1
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
