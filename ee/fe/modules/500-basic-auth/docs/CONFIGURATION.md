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
* `highAvailability` — manually enable/disable the high availability mode. By default, the high availability mode is determined automatically. Click [here](../../deckhouse-configure-global.html#parameters) to learn more about the HA mode for modules.
* `locations` —  add this parameter if you need to create multiple locations for various applications with different authorization;
    * `location` — the location for which `whitelist` and `users` are specified (in the nginx config, the `root` is replaced with `/`;
    * `whitelist` — a list of IP addresses and subnets for which no login/password is required for authorization;
    * `users` — a list of users in the `username: "password"` format;
* `nodeSelector` — the same as in the pods' `spec.nodeSelector` parameter in Kubernetes;
    * If the parameter is omitted or `false`, it will be determined [automatically](../../#advanced-scheduling).
* `tolerations` — the same as in the pods' `spec.tolerations` parameter in Kubernetes;
    * If the parameter is omitted or `false`, it will be determined [automatically](../../#advanced-scheduling).
