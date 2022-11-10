---
title: "The deckhouse-web module: examples"
---

## An example of the module configuration

Below is a simple example of the module configuration:

```yaml
deckhouseWeb: |
  nodeSelector:
    node-role/example: ""
  tolerations:
  - key: dedicated
    operator: Equal
    value: example
  externalAuthentication:
    authURL: "https://<applicationDomain>/auth"
    authSignInURL: "https://<applicationDomain>/sign-in"
    authResponseHeaders: "Authorization"
```
