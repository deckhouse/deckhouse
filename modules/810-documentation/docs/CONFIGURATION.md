---
title: "The documentation module: configuration"
---

The module does not have any mandatory parameters.

<!-- SCHEMA -->

## Authentication

[user-authn](/documentation/v1/modules/150-user-authn/) module provides authentication by default. Also, externalAuthentication can be configured (see below).
If these options are disabled, the module will use basic auth with the auto-generated password.

Use kubectl to see password:

```shell
kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module values documentation -o json | jq '.internal.auth.password'
```

Delete the Secret to re-generate password:

```shell
kubectl -n d8-system delete secret/documentation-basic-auth
```

> **Note!** The `auth.password` parameter is deprecated.
