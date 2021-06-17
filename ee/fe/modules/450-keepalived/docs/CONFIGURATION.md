---
title: "The keepalived module: configuration"
---

This module is **disabled** by default. To enable it, add the following lines to the `deckhouse` ConfigMap:

```yaml
data:
  keepalivedEnabled: "true"
```

The module does **not** have any parameters in the `deckhouse` ConfigMap.

Keepalived clusters are configured using the [Custom Resource](cr.html).
