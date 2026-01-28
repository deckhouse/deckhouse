---
title: "Cluster SLA Monitoring: configuration"
---

<!-- SCHEMA -->

## Authentication

[user-authn](/modules/user-authn/) module provides authentication by default. Also, externalAuthentication can be configured (see below).
If these options are disabled, the module will use basic auth with the auto-generated password.

Use d8 k to see password:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values upmeter -o json | jq '.upmeter.internal.auth.webui.password'
```

Delete the Secret to re-generate password:

```shell
d8 k -n d8-upmeter delete secret/basic-auth-webui
```

Use d8 k to see password for status page:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values upmeter -o json | jq '.upmeter.internal.auth.status.password'
```

Delete the Secret to re-generate password for status page:

```shell
d8 k -n d8-upmeter delete secret/basic-auth-status
```

> **Note!** The `auth.status.password` and `auth.webui.password` parameters are deprecated.
