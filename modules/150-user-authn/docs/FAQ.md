---
title: "The user-authn module: FAQ"
---

## How do I turn off authentication for trusted addresses?

Use the annotations:
```yaml
nginx.ingress.kubernetes.io/whitelist-source-range: 192.168.0.0/32,1.1.1.1`
nginx.ingress.kubernetes.io/satisfy: "any"
```

You can learn more [here](usage.html#setting-up-cidr-based-restrictions)...
