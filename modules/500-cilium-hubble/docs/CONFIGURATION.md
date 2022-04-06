---
title: "The cilium-hubble module: configuration"
---

The module is **automatically** enabled when `cni-cilium` is used.
To disable this module you can add to the `deckhouse` ConfigMap:
```
ciliumHubbleEnabled: "false"
```

## Parameters

<!-- SCHEMA -->

