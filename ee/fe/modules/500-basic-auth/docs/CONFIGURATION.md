---
title: "The basic-auth module: configuration"
---

This module is **disabled** by default. To enable it, add the following lines to the `deckhouse` ConfigMap:

```yaml
data:
  basicAuthEnabled: "true"
```

The module does not have any mandatory settings.
By default, it creates the `/` location with the `admin` user.

## Parameters

<!-- SCHEMA -->
