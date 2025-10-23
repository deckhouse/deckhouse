---
title: "Authentication"
permalink: en/stronghold/documentation/user/concepts/auth.html
lang: en
description: >-
  Before performing any operation with Stronghold, the connecting client must be
  authenticated.
---

Authentication in Stronghold is the process by which user or machine supplied information is verified against an internal or external system.
Stronghold supports multiple auth methods including Userpass, LDAP, AppRole, and more.
Each auth method has a specific use case.

Before interacting with Stronghold, each client must _authenticate_ using one of the supported authentication methods.
Upon successful authentication, a token is generated, which is conceptually similar to a session ID on a website.
The token may have attached policy, which is mapped at the authentication time.
For details on this process, refer to [Policies](policy.html).

## Authentication methods

Stronghold supports a number of authentication methods.
Some backends are targeted toward users while others are targeted toward machines.
Most authentication backends must be enabled before use.

To enable an authentication method, run the following command:

```shell
d8 stronghold write sys/auth/my-auth type=userpass
```

This enables the `userpass` authentication method at the `my-auth` path.
This authentication will be accessible at the `my-auth` path.
Often you will see authentications at the same path as their name, but this is not a requirement.

To learn more about this authentication, use the built-in `path-help` command:

```shell
d8 stronghold path-help auth/my-auth
```

Stronghold supports multiple authentication methods simultaneously,
and you can even mount the same type of authentication method at different paths.
Only one authentication is required to gain access to Stronghold,
and it is not currently possible to force a user through multiple auth methods to gain access,
although some backends do support the multifactor authentication (MFA).

## Tokens

Authentication works by verifying your identity and then generating a token to associate with that identity.

For example, even though you may authenticate using DEX, Stronghold generates a unique access token for you to use for future requests.
The CLI automatically attaches this token to requests, but if you're using the API you'll have to do this manually.

This token given for authentication with any backend can also be used with the full set of token commands,
such as creating new sub-tokens, revoking tokens, and renewing tokens.
For details, refer to [Tokens](tokens.html).

## Authentication

### Via the CLI

To authenticate with the CLI, `d8 stronghold login` is used.
This supports many of the built-in authentication methods.

For example, to authenticate with OIDC, run the following command:

```shell
d8 stronghold login -method=oidc
```

After authenticating, you will be logged in.
The CLI command will also output your raw token.
This token is used for revocation and renewal.
As the user logging in, the primary use case of the token is renewal, which is covered below in [Authentication leases](#authentication-leases).

To determine what variables are needed for an auth method,
supply the `-method` flag without any additional arguments and help will be shown.

If you're using a method that isn't supported via the CLI, then the API must be used.

### Via the API

API authentication is generally used for machine authentication.
Each authentication method implements its own login endpoint.

To find the proper endpoint, use the following command:

```shell
d8 stronghold path-help
```

## Authentication leases

Just like secrets, identities have [leases](lease.html) associated with them.
This means that you must reauthenticate after the given lease period to continue accessing Stronghold.

To set the lease associated with an identity, reference the documentation for the specific auth method in use.
It is specific to each backend how leasing is implemented.

And just like secrets, identities can be renewed without having to completely reauthenticate.
To renew it, use the following command, specifying the token associated with your identity:

```shell
d8 stronghold token renew <token>
```
