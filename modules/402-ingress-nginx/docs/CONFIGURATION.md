---
title: "The ingress-nginx module: configuration"
---

This module is **enabled** by default in clusters from version 1.14 onward. To disable it, add the following lines to the `d8-system/deckhouse` ConfigMap:
```yaml
ingressNginxEnabled: "false"
```

## Parameters

* `defaultControllerVersion` — the version of the ingress-nginx controller that is used for all controllers by default if the `controllerVersion` parameter is omitted in the IngressNginxController CR.
    * The default version is `0.33`,
    * Available alternatives are `0.25`, `0.26`, `0.33`, `0.46`.


Ingress controllers are configured using the [IngressNginxController](cr.html#ingressnginxcontroller) Custom Resource.
