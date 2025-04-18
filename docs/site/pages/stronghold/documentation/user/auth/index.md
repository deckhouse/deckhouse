---
sidebar_label: Overview
description: Auth methods are mountable methods that perform authentication for Stronghold.
---

# Auth methods

Auth methods are the components in Stronghold that perform authentication and are
responsible for assigning identity and a set of policies to a user. In all cases,
Stronghold will enforce authentication as part of the request processing. In most cases,
Stronghold will delegate the authentication administration and decision to the relevant configured
external auth method (e.g., Kubernetes).

Having multiple auth methods enables you to use an auth method that makes the
most sense for your use case of Stronghold and your organization.

For example, on developer machines, the [Userpass](/docs/auth/userpass)
is easiest to use. But for servers the [AppRole](/docs/auth/approle)
method is the recommended choice.

To learn more about authentication, see the
[authentication concepts page](/docs/concepts/auth).

## Enabling/Disabling auth methods

Auth methods can be enabled/disabled using the CLI or the API.

```shell-session
d8 stronghold auth enable userpass
```

When enabled, auth methods are similar to [secrets engines](/docs/secrets):
they are mounted within the Stronghold mount table and can be accessed
and configured using the standard read/write API. All auth methods are mounted underneath the `auth/` prefix.

By default, auth methods are mounted to `auth/<type>`. For example, if you
enable "ldap", then you can interact with it at `auth/ldap`. However, this
path is customizable, allowing users with advanced use cases to mount a single
auth method multiple times.

```shell-session
d8 stronghold auth enable -path=my-login userpass
```

When an auth method is disabled, all users authenticated via that method are
automatically logged out.

## External auth method considerations

When using an external auth method (e.g., Kubernetes), Stronghold will call the external service
at the time of authentication and for subsequent token renewals. If the status
of an entity changes in the external system (e.g., an account expires or is
disabled), Stronghold denies requests to **renew** tokens associated with the entity.
However, any existing token remain valid for the original grant period unless
they are explicitly revoked within Stronghold. Operators should set appropriate
[token TTLs](/docs/concepts/tokens#the-general-case) when using external
authN methods.
