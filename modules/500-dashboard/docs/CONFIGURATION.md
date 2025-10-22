---
title: "The dashboard module: configuration"
---

## Authentication

By default, the [user-authn](/modules/user-authn/) module is used. Alternatively, authentication can be configured using [`externalAuthentication`](examples.html).

If neither of these methods is enabled, the `dashboard` module will be disabled.

{% alert level="warning" %}
The parameters `auth.password` and `accessLevel` are no longer supported.
{% endalert %}

## Settings

The module does not have any mandatory parameters.

<!-- SCHEMA -->
