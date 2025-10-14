---
title: "Authentication"
permalink: en/stronghold/documentation/user/concepts/auth.html
lang: en
description: >-
  Before performing any operation with Stronghold, the connecting client must be
  authenticated.
---

Authentication in Stronghold is the process by which user or machine supplied
information is verified against an internal or external system. Stronghold supports
multiple auth methods including GitHub,
LDAP, AppRole, and more. Each auth method has a specific use case.

Before a client can interact with Stronghold, it must _authenticate_ against an
auth method. Upon authentication, a token is generated. This token is
conceptually similar to a session ID on a website. The token may have attached
policy, which is mapped at authentication time. This process is described in
detail in the [policies concepts](policy.html) documentation.

## Auth methods

Stronghold supports a number of auth methods. Some backends are targeted
toward users while others are targeted toward machines. Most authentication
backends must be enabled before use. To enable an auth method:

```shell-session
d8 stronghold write sys/auth/my-auth type=userpass
```

This enables the "userpass" auth method at the path "my-auth". This
authentication will be accessible at the path "my-auth". Often you will see
authentications at the same path as their name, but this is not a requirement.

To learn more about this authentication, use the built-in `path-help` command:

```shell-session
$ d8 stronghold path-help auth/my-auth
# ...
```

Stronghold supports multiple auth methods simultaneously, and you can even
mount the same type of auth method at different paths. Only one
authentication is required to gain access to Stronghold, and it is not currently
possible to force a user through multiple auth methods to gain
access, although some backends do support MFA.

## Tokens

There is an [entire page dedicated to tokens](tokens.html),
but it is important to understand that authentication works by verifying
your identity and then generating a token to associate with that identity.

For example, even though you may authenticate using something like GitHub,
Stronghold generates a unique access token for you to use for future requests.
The CLI automatically attaches this token to requests, but if you're using
the API you'll have to do this manually.

This token given for authentication with any backend can also be used
with the full set of token commands, such as creating new sub-tokens,
revoking tokens, and renewing tokens. This is all covered on the
[token concepts page](tokens.html).

## Authenticating

### Via the CLI

To authenticate with the CLI, `d8 stronghold login` is used. This supports many
of the built-in auth methods. For example, with GitHub:

```shell-session
$ d8 stronghold login -method=github token=<token>
...
```

After authenticating, you will be logged in. The CLI command will also
output your raw token. This token is used for revocation and renewal.
As the user logging in, the primary use case of the token is renewal,
covered below in the "Auth Leases" section.

To determine what variables are needed for an auth method,
supply the `-method` flag without any additional arguments and help
will be shown.

If you're using a method that isn't supported via the CLI, then the API
must be used.

### Via the API

API authentication is generally used for machine authentication. Each
auth method implements its own login endpoint. Use the `d8 stronghold path-help`
mechanism to find the proper endpoint.

For example, the GitHub login endpoint is located at `auth/github/login`.
And to determine the arguments needed, `d8 stronghold path-help auth/github/login` can
be used.

## Auth leases

Just like secrets, identities have
[leases](lease.html) associated with them. This means that
you must reauthenticate after the given lease period to continue accessing
Stronghold.

To set the lease associated with an identity, reference the help for
the specific auth method in use. It is specific to each backend
how leasing is implemented.

And just like secrets, identities can be renewed without having to
completely reauthenticate. Just use `d8 stronghold token renew <token>` with the
leased token associated with your identity to renew it.
