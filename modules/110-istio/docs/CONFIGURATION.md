---
title: "The istio module: configuration"
---

<!-- SCHEMA -->

## Authentication

[user-authn](../150-user-authn/) module provides authentication by default. Also, externalAuthentication can be configured (see below).
If these options are disabled, the module will use basic auth with the auto-generated password.

Use kubectl to see password:

```shell
kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module values istio -o json | jq '.istio.internal.auth.password'
```

Delete secret to re-generate password:

```shell
kubectl -n d8-istio delete secret/kiali-basic-auth
```

> **Note!** The `auth.password` parameter is deprecated.
