---
title: "OIDC"
permalink: en/stronghold/documentation/user/auth/oidc.html
lang: en

description: >-
  The JWT/OIDC auth method allows authentication using OIDC and user-provided JWTs
---

### JWT/OIDC auth method

The `jwt` auth method can be used to authenticate with Stronghold using
OIDC or by providing a JWT.

The OIDC method allows authentication via a configured OIDC provider using the
user's web browser. This method may be initiated from the Stronghold UI or the
command line. Alternatively, a JWT can be provided directly. The JWT is
cryptographically verified using locally-provided keys, or, if configured, an
OIDC Discovery service can be used to fetch the appropriate keys. The choice of
method is configured per role.

Both methods allow additional processing of the claims data in the JWT. Some of
the concepts common to both methods will be covered first, followed by specific
examples of OIDC and JWT usage.

## OIDC authentication

This section covers the setup and use of OIDC roles. Basic
familiarity with [OIDC concepts](https://developer.okta.com/blog/2017/07/25/oidc-primer-part-1)
is assumed. The Authorization Code flow makes use of the Proof Key for Code
Exchange (PKCE) extension.

Stronghold includes two built-in OIDC login flows: the Stronghold UI, and the CLI
using a `d8 stronghold login`.

### Redirect URIs

Unless you are using `callbackmode=device`,
an important part of OIDC role configuration is properly setting redirect URIs. This must be
done both in Stronghold and with the OIDC provider, and these configurations must align. The
redirect URIs are specified for a role with the `allowed_redirect_uris` parameter. There are
different redirect URIs to configure the Stronghold UI and CLI flows, so one or both will need to
be set up depending on the installation.

#### CLI

If you plan to support authentication via `d8 stronghold login -method=oidc` and
are not using `callbackmode=device`, a redirect URI with a path ending
in `oidc/callback` must be set. With the default `callbackmode=client`
this can usually be `http://localhost:8250/oidc/callback`.
With `callbackmode=direct` this should be a URI of the form:

`https://{host:port}/v1/auth/{path}/oidc/callback`

where "host:port" is the Stronghold server name and port, and "path" is the path the JWT
backend is mounted at (e.g. "oidc" or "jwt").

Logins via the CLI may
specify a different host and/or listening port if needed, and a URI with this host/port must match one
of the configured redirected URIs. These same URIs must be added to the provider as well.

### Stronghold UI

Logging in using Deckhouse Stronghold doesn't require manually configuring the UI. It's configured automatically when Deckhouse Stronghold is enabled.

### OIDC login (Stronghold UI)

1. Select the "OIDC" login method.
1. Enter a role name if necessary.
1. Press "Sign In" and complete the authentication with the configured provider.

### OIDC login (CLI)

The CLI login defaults to path of `/oidc`. If this auth method was enabled at a
different path, specify `-path=/my-path` in the CLI.

```shell-session
$ d8 stronghold login -method=oidc port=8400 role=test

Complete the login via your OIDC provider. Launching browser to:

    https://myco.auth0.com/authorize?redirect_uri=http%3A%2F%2Flocalhost%3A8400%2Foidc%2Fcallback&client_id=r3qXc2bix9eF...
```

The browser will open to the generated URL to complete the provider's login. The
URL may be entered manually if the browser cannot be automatically opened.

- `skip_browser` (default: "false"). Toggle the automatic launching of the default browser to the login URL.

The callback listener may be customized with the following optional parameters. These are typically
not required to be set:

- `mount` (default: "oidc")
- `callbackmode` (default: "client").  Mode of callback:
   "client" for connection to a port on the cli client,
   `direct` for direct connection to the Stronghold server,
   or "device" for device flow which has no callback.
- `listenaddress` (default: "localhost").  Only for `client` callback mode.
- `port` (default: 8250).  Only for `client` callback mode.
- `callbackhost` (default: the Stronghold's server and port in direct callback mode, else "localhost")
- `callbackmethod` (default: the method used for the Stronghold server in direct callback mode, else "http").
   The method to use in an OIDC `redirect_uri`.
- `callbackport` (default: value set for `port` in client callback mode, otherwise the port of the Stronghold
   server and an added `/v1/auth/<path>` where `<path>` is from the login -path option)
   This value is used in the `redirect_uri`, whereas
  `port` is the localhost port that the listener is using. These two may be different in advanced setups.

### OIDC provider configuration

The OIDC authentication flow has been successfully tested with a number of providers. A full
guide to configuring OAuth/OIDC applications is beyond the scope of Stronghold documentation.

### OIDC configuration troubleshooting

This amount of configuration required for OIDC is relatively small, but it can be tricky to debug
why things aren't working. Some tips for setting up OIDC:

- If a role parameter (e.g. `bound_claims`) requires a map value, it can't be set individually using
  the Stronghold CLI. In these cases the best approach is to write the entire configuration as a single
  JSON object:

```text
d8 stronghold write auth/oidc/role/demo -<<EOF
{
  "user_claim": "sub",
  "bound_audiences": "abc123",
  "role_type": "oidc",
  "policies": "demo",
  "ttl": "1h",
  "bound_claims": { "groups": ["mygroup/mysubgroup"] }
}
EOF
```

- Monitor Stronghold's log output. Important information about OIDC validation failures will be emitted.

- Ensure Redirect URIs are correct in Stronghold and on the provider. They need to match exactly. Check:
  http/https, 127.0.0.1/localhost, port numbers, whether trailing slashes are present.

- Start simple. The only claim configuration a role requires is `user_claim`. After authentication is
  known to work, you can add additional claims bindings and metadata copying.

- `bound_audiences` is optional for OIDC roles and typically not required. OIDC providers will use
  the client_id as the audience and OIDC validation expects this.

- Check your provider for what scopes are required in order to receive all
  of the information you need. The scopes "profile" and "groups" often need to be
  requested, and can be added by setting `oidc_scopes="profile,groups"` on the role.

- If you're seeing claim-related errors in logs, review the provider's docs very carefully to see
  how they're naming and structuring their claims. Depending on the provider, you may be able to
  construct a simple `curl` implicit grant request to obtain a JWT that you can inspect. An example
  of how to decode the JWT (in this case located in the "access_token" field of a JSON response):

  `cat jwt.json | jq -r .access_token | cut -d. -f2 | base64 -D`

- The `verbose_oidc_logging` role
  option is available which will log the received OIDC token to the _server_ logs if debug-level logging is enabled. This can
  be helpful when debugging provider setup and verifying that the received claims are what you expect.
  Since claims data is logged verbatim and may contain sensitive information, this option should not be
  used in production.
