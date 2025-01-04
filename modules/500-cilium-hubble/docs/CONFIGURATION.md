---
title: "The cilium-hubble module: configuration"
---

{% include module-alerts.liquid %}

{% include module-bundle.liquid %}

The module will be left disabled unless `cni-cilium` is used regardless of `ciliumHubbleEnabled:` parameter.

{% include module-settings.liquid %}

## Authentication

[user-authn](/products/kubernetes-platform/documentation/v1/modules/user-authn/) module provides authentication by default. Also, externalAuthentication can be configured (see below).
If these options are disabled, the module will use basic auth with the auto-generated password.

Use kubectl to see password:

```shell
kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values cilium-hubble -o json | jq '.ciliumHubble.internal.auth.password'
```

Delete the Secret to re-generate password:

```shell
kubectl -n d8-cni-cilium delete secret/hubble-basic-auth
```

> **Note!** The `auth.password` parameter is deprecated.
