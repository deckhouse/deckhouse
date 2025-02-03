---
title: "The cni-simple-bridge module: configuration"
---

The module does not have any settings, but in order to use it, you need to explicitly enable it by using the 'ModuleConfig'.

To enable/disable the module, set spec.enabled field of the ModuleConfig custom resource to true or false. Note that this may require you to first create a ModuleConfig resource for the module.

## An example of the configuration

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-simple-bridge
spec:
  enabled: true
  version: 1
```

<!-- SCHEMA -->
