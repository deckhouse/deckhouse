---
title: "The ingress-nginx module: configuration"
---

This module is **enabled** by default in clusters from version 1.14 onward. To disable it, add the following lines to the `d8-system/deckhouse` ConfigMap:
```yaml
ingressNginxEnabled: "false"
```

## Parameters

<!-- SCHEMA -->

Ingress controllers are configured using the [IngressNginxController](cr.html#ingressnginxcontroller) Custom Resource.
