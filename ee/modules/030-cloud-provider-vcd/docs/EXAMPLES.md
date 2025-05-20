---
title: "Cloud provider — VMware Cloud Director: examples"
---

Below is an example configuration [`VCDInstanceClass`](cr.html#vcdinstanceclass) for ephemeral nodes at VMware Cloud Director cloud provider.

## An example of the `VCDInstanceClass` custom resource

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
