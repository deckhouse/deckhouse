---
title: "JWT method"
permalink: en/stronghold/documentation/user/auth/jwt.html
lang: en
description: >-
  The JWT/OIDC auth method allows authentication using OIDC and user-provided
  JWTs
---

## JWT authentication

The authentication flow for roles of type "jwt" is simpler than OIDC since Stronghold
only needs to validate the provided JWT.

### JWT verification

JWT signatures will be verified against public keys from the issuer. This process can be done in
three different ways, though only one method may be configured for a single backend:

- **Static Keys**. A set of public keys is stored directly in the backend configuration.

- **JWKS**. A JSON Web Key Set ([JWKS](https://tools.ietf.org/html/rfc7517)) URL (and optional
  certificate chain) is configured. Keys will be fetched from this endpoint during authentication.

- **OIDC Discovery**. An OIDC Discovery URL (and optional certificate chain) is configured. Keys
  will be fetched from this URL during authentication. When OIDC Discovery is used, OIDC validation
  criteria (e.g. `iss`, `aud`, etc.) will be applied.

If multiple methods are needed, another instance of the backend can be mounted and configured
at a different path.

### Via the CLI

The default path is `/jwt`. If this auth method was enabled at a
different path, specify `-path=/my-path` in the CLI.

```shell-session
d8 stronghold write auth/jwt/login role=demo jwt=...
```

### Via the API

The default endpoint is `auth/jwt/login`. If this auth method was enabled
at a different path, use that value instead of `jwt`.

```shell-session
$ curl \
    --request POST \
    --data '{"jwt": "your_jwt", "role": "demo"}' \
    http://127.0.0.1:8200/v1/auth/jwt/login
```

The response will contain a token at `auth.client_token`:

```json
{
  "auth": {
    "client_token": "38fe9691-e623-7238-f618-c94d4e7bc674",
    "accessor": "78e87a38-84ed-2692-538f-ca8b9f400ab3",
    "policies": ["default"],
    "metadata": {
      "role": "demo"
    },
    "lease_duration": 2764800,
    "renewable": true
  }
}
```

## Configuration

Auth methods must be configured in advance before users or machines can
authenticate. These steps are usually completed by an operator or configuration
management tool.

1. Enable the JWT auth method. Either the "jwt" or "oidc" name may be used. The
   backend will be mounted at the chosen name.

   ```text
   $ d8 stronghold auth enable jwt
     or
   $ d8 stronghold auth enable oidc
   ```

1. Use the `/config` endpoint to configure Stronghold. To support JWT roles, either local keys, a JWKS URL, or an OIDC
   Discovery URL must be present. For OIDC roles, OIDC Discovery URL, OIDC Client ID and OIDC Client Secret are required.

   ```text
   $ d8 stronghold write auth/jwt/config \
       oidc_discovery_url="https://myco.auth0.com/" \
       oidc_client_id="m5i8bj3iofytj" \
       oidc_client_secret="f4ubv72nfiu23hnsj" \
       default_role="demo"
   ```

   If you need to perform JWT verification with JWT token validation, then leave the `oidc_client_id` and `oidc_client_secret` blank.

   ```text
   $ d8 stronghold write auth/jwt/config \
      oidc_discovery_url="https://MYDOMAIN.eu.auth0.com/" \
      oidc_client_id="" \
      oidc_client_secret="" \
   ```

1. Create a named role:

   ```text
   d8 stronghold write auth/jwt/role/demo \
       allowed_redirect_uris="http://localhost:8250/oidc/callback" \
       bound_subject="r3qX9DljwFIWhsiqwFiu38209F10atW6@clients" \
       bound_audiences="https://vault.plugin.auth.jwt.test" \
       user_claim="https://vault/user" \
       groups_claim="https://vault/groups" \
       policies=webapps \
       ttl=1h
   ```

   This role authorizes JWTs with the given subject and audience claims, gives
   it the `webapps` policy, and uses the given user/groups claims to set up
   Identity aliases.

   For the complete list of configuration options, please see the API
   documentation.

### Bound claims

Once a JWT has been validated as being properly signed and not expired, the
authorization flow will validate that any configured "bound" parameters match.
In some cases there are dedicated parameters, for example `bound_subject`,
which must match the JWT's `sub` parameter. A role may also be configured to
check arbitrary claims through the `bound_claims` map. The map contains a set
of claims and their required values. For example, assume `bound_claims` is set
to:

```json
{
  "division": "Europe",
  "department": "Engineering"
}
```

Only JWTs containing both the "division" and "department" claims, and
respective matching values of "Europe" and "Engineering", would be authorized.
If the expected value is a list, the claim must match one of the items in the list.
To limit authorization to a set of email addresses:

```json
{
  "email": ["fred@example.com", "julie@example.com"]
}
```

Bound claims can optionally be configured with globs.

### Claims as metadata

Data from claims can be copied into the resulting auth token and alias metadata by configuring `claim_mappings`. This role
parameter is a map of items to copy. The map elements are of the form: `"<JWT claim>":"<metadata key>"`. Assume
`claim_mappings` is set to:

```json
{
  "division": "organization",
  "department": "department"
}
```

This specifies that the value in the JWT claim "division" should be copied to the metadata key "organization". The JWT
"department" claim value will also be copied into metadata but will retain the key name. If a claim is configured in `claim_mappings`,
it must existing in the JWT or else the authentication will fail.

Note: the metadata key name "role" is reserved and may not be used for claim mappings.

### Claim specifications and JSON pointer

Some parameters (e.g. `bound_claims`, `groups_claim`, `claim_mappings`, `user_claim`) are
used to point to data within the JWT. If the desired key is at the top of level of the JWT,
the name can be provided directly. If it is nested at a lower level, a JSON Pointer may be
used.

Assume the following JSON data to be referenced:

```json
{
  "division": "North America",
  "groups": {
    "primary": "Engineering",
    "secondary": "Software"
  }
}
```

A parameter of `"division"` will reference "North America", as this is a top level key. A parameter
`"/groups/primary"` uses JSON Pointer syntax to reference "Engineering" at a lower level. Any valid
JSON Pointer can be used as a selector. Refer to the
[JSON Pointer RFC](https://tools.ietf.org/html/rfc6901) for a full description of the syntax.
