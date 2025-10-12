---
title: "The cilium-hubble module: configuration"
---

{% include module-alerts.liquid %}

{% include module-bundle.liquid %}

If the `cni-cilium` module is disabled, the `ciliumHubbleEnabled:` parameter will not affect the enabling of the `cilium-hubble` module.

{% include module-conversion.liquid %}

{% include module-settings.liquid %}

## Authentication

[user-authn](/modules/user-authn/) module provides authentication by default. Also, externalAuthentication can be configured.
If these options are disabled, the module will use basic auth with the auto-generated password.

To view the generated password, run the command:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values cilium-hubble -o json | jq '.ciliumHubble.internal.auth.password'
```

To generate a new password, delete the Secret:

```shell
d8 k -n d8-cni-cilium delete secret/hubble-basic-auth
```

{% alert level="info" %}
The `auth.password` parameter is deprecated.
{% endalert %}
