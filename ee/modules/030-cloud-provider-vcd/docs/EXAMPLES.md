---
title: "Cloud provider — vCloud Director: examples"
---

Below is an example configuration for a vCloud Director cloud provider.

## An example of the `VsphereInstanceClass` custom resource

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
