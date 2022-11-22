---
title: "The openvpn module: configuration"
---

{% include module-bundle.liquid %}

> **Caution!** The admin panel always uses a subnet defined in the `tunnelNetwork` parameter. Static user addresses must be issued from this subnet. If the UDP protocol is used, these addresses will be converted for use in `udpTunnelNetwork` subnet. In this case, the networks in the `tunnelNetwork` and `udpTunnelNetwork` parameters must be the same size.
>
> Example:
> * `tunnelNetwork`: `10.5.5.0/24`
> * `udpTunnelNetwork`: `10.5.6.0/24`
> * IP ddress for user `10.5.5.8` (from the `tunnelNetwork` CIDR) will be converted to `10.5.6.8` (from the `udpTunnelNetwork` CIDR).

## Authentication

[user-authn](../150-user-authn/) module provides authentication by default. You can also configure authentication using the [externalAuthentication](#parameters-auth-externalauthentication) parameter. If these options are disabled, the module will use basic auth with the auto-generated password.

Use kubectl to see password:

```shell
kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module values openvpn -o json | jq '.openvpn.internal.auth.password'
```

Delete secret to re-generate password:

```shell
kubectl -n d8-openvpn delete secret/basic-auth
```

> **Note!** The `auth.password` parameter is deprecated.

## Parameters

<!-- SCHEMA -->
