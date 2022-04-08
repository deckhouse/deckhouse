---
title: "The ceph-csi module: configuration"
---

The module does not require any configuration and is **disabled** by default. To enable it, add the following lines to the `deckhouse` ConfigMap:

```yaml
data:
  cephCsiEnabled: "true"
```
