---
title: "The cilium-hubble module: configuration"
---

This module is **disabled** by default.

To enable this module you can add to the `deckhouse` ConfigMap:

```yaml
ciliumHubbleEnabled: "true"
```

The module will be left disabled unless `cni-cilium` is used regardless of `ciliumHubbleEnabled:` parameter.

## Parameters

<!-- SCHEMA -->
