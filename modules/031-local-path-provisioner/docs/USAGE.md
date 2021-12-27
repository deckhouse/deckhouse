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

Notes:

- This example will create `localpath-system` storage class which **must** be used by pods for everything to work
- Volumes created by provisioner will have delete retention policy which is hardcoded ([issue](https://github.com/deckhouse/deckhouse/issues/360))
- If provisioner will be delete before claims folders will not be deleted from node
- Note that in example `system` node is used which probably will have some taints so pods **must** have corresponding tolerations
