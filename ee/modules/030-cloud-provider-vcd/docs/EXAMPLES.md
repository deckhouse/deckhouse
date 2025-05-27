---
title: "Cloud provider — VMware Cloud Director: examples"
---

Below is an example [VCDInstanceClass](cr.html#vcdinstanceclass) configuration for ephemeral nodes of VMware Cloud Director cloud provider.

## Example configuration of the VCDInstanceClass custom resource

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
