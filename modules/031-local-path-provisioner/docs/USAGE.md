---
title: "The local-path-provisioner module: configuration examples"
---

## Example CR `LocalPathProvisioner`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath-system
spec:
  nodeGroups:
  - system
  path: "/opt/local-path-provisioner"
```
