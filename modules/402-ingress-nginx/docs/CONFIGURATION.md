---
title: "The ingress-nginx module: configuration"
---

This module is **enabled** by default in clusters from version 1.14 onward. To disable it, add the following lines to the `d8-system/deckhouse` ConfigMap:
```yaml
ingressNginxEnabled: "false"
```

> Pay attention to the global parameter [publicDomainTemplate](../../deckhouse-configure-global.html#parameters), if you are turning the module on. If the parameter is not specified, the Ingress resources for Deckhouse service components (dashboard, user-auth, grafana, upmeter, etc.) will not be created.

## Parameters

<!-- SCHEMA -->

Ingress controllers are configured using the [IngressNginxController](cr.html#ingressnginxcontroller) Custom Resource.
