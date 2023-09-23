---
title: "The upmeter module: configuration"
---

<!-- SCHEMA -->

## Authentication

[user-authn](/documentation/v1/modules/150-user-authn/) module provides authentication by default. Also, externalAuthentication can be configured (see below).
If these options are disabled, the module will use basic auth with the auto-generated password.

Use kubectl to see password:

```shell
kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module values upmeter -o json | jq '.upmeter.internal.auth.webui.password'
```

Delete the Secret to re-generate password:

```shell
kubectl -n d8-upmeter delete secret/basic-auth-webui
```

Use kubectl to see password for status page:

```shell
kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module values upmeter -o json | jq '.upmeter.internal.auth.status.password'
```

Delete the Secret to re-generate password for status page:

```shell
kubectl -n d8-upmeter delete secret/basic-auth-status
```

> **Note!** The `auth.status.password` and `auth.webui.password` parameters are deprecated.
