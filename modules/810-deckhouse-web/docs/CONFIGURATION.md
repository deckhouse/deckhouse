---
title: "The deckhouse-web module: configuration"
---

{% include module-bundle.liquid %}

The module does not have any mandatory parameters.

## Authentication

[user-authn](/documentation/v1/modules/150-user-authn/) module provides authentication by default. Also, externalAuthentication can be configured (see below).
If these options are disabled, the module will use basic auth with the auto-generated password.

Use kubectl to see password:

```shell
kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module values deckhouse-web -o json | jq '.deckhouseWeb.internal.auth.password'
```

Delete secret to re-generate password:

```shell
kubectl -n d8-system delete secret/deckhouse-web-basic-auth
```

> **Note!** The `auth.password` parameter is deprecated.

## Parameters

<!-- SCHEMA -->
