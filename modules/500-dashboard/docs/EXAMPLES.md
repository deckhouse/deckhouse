---
title: "The dashboard module: examples"
---

## An example of the module configuration

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: dashboard
spec:
  version: 1
  enabled: true
  settings:
    nodeSelector:
      node-role/system: ""
    tolerations:
    - key: dedicated.deckhouse.io
      operator: Equal
      value: system
    externalAuthentication:
      authURL: "https://<applicationDomain>/auth"
      authSignInURL: "https://<applicationDomain>/sign-in"
      authResponseHeaders: "Authorization"
```
