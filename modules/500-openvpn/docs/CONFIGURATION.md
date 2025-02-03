---
title: "The openvpn module: configuration"
---

<!-- SCHEMA -->

## Authentication

[user-authn](../user-authn/) module provides authentication by default. You can also configure authentication using the [externalAuthentication](#parameters-auth-externalauthentication) parameter. If these options are disabled, the module will use basic auth with the auto-generated password.

Use kubectl to see password:

```shell
kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values openvpn -o json | jq '.openvpn.internal.auth.password'
```

Delete the Secret to re-generate password:

```shell
kubectl -n d8-openvpn delete secret/basic-auth
```

> **Note!** The `auth.password` parameter is deprecated.
