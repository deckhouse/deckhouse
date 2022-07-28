---
title: "The istio module: configuration"
---

This module is **disabled** by default. To enable it, add the following lines to the `deckhouse` ConfigMap:

```yaml
data:
  istioEnabled: "true"
```

## Authentication

[user-authn](/{{ page.lang }}/documentation/v1/modules/150-user-authn/) module provides authentication by default. Also, externalAuthentication can be configured (see below).
If these options are disabled, the module will use basic auth with the auto-generated password.

Use kubectl to see password:

```shell
kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module values istio -o json | jq '.istio.internal.auth.password'
```

Delete secret to re-generate password:

```shell
kubectl -n d8-istio delete secret/kiali-basic-auth
```

**Note:** auth.password parameter is deprecated.

## Parameters

<!-- SCHEMA -->
