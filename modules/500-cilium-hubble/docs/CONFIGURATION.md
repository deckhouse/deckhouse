---
title: "The cilium-hubble module: configuration"
---

This module is **disabled** by default.

To enable this module you can add to the `deckhouse` ConfigMap:

```yaml
ciliumHubbleEnabled: "true"
```

The module will be left disabled unless `cni-cilium` is used regardless of `ciliumHubbleEnabled:` parameter.

## Authentication

[user-authn](/{{ page.lang }}/documentation/v1/modules/150-user-authn/) module provides authentication by default. Also, externalAuthentication can be configured (see below).
If these options are disabled, the module will use basic auth with the auto-generated password.

Use kubectl to see password:

```shell
kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module values cilium-hubble -o json | jq '.ciliumHubble.internal.auth.password'
```

Delete secret to re-generate password:

```shell
kubectl -n d8-cni-cilium delete secret/hubble-basic-auth
```

**Note:** auth.password parameter is deprecated.

## Parameters

<!-- SCHEMA -->
