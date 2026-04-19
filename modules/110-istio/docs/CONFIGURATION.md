---
title: "The istio module: configuration"
---

<!-- SCHEMA -->

## Authentication

[user-authn](../user-authn/) module provides authentication by default. Also, externalAuthentication can be configured (see below).
If these options are disabled, the module will use basic auth with the auto-generated password.

Use d8 k to see password:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values istio -o json | jq '.istio.internal.auth.password'
```

Delete the Secret to re-generate password:

```shell
d8 k -n d8-istio delete secret/kiali-basic-auth
```

> **Note!** The `auth.password` parameter is deprecated.
