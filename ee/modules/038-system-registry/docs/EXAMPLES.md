---
title: "Module registry: examples"
description: ""
---

## Switching to `Direct` mode

To switch an already running cluster to `Direct` mode, follow these steps:

1. Make sure the `registry` module is enabled and running.

```bash
kubectl get module registry
```

2. Add the following settings to the `ModuleConfig` of the `deckhouse` module:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    registry:
      mode: Direct
      direct:
        imagesRepo: registry.deckhouse.ru/deckhouse/ee
        scheme: https
        license: <LICENSE_KEY> # Replace with your license key
    ...
```

{% alert level="warning" %}
If you are using a registry other than `registry.deckhouse.ru`, refer to the [`deckhouse`](https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/deckhouse/) module configuration for proper setup.
{% endalert %}

3. Check the status of the `registry-state` secret by reading the [guide](./faq.html#how-to-check-the-registry-mode-switch-status).
