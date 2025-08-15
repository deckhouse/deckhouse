---
title: "External Vault integration"
menuTitle: External Vault integration
force_searchable: true
description: Resolving secrets from an external Vault in CI
weight: 50
permalink: en/code/documentation/user/external-vault.html
lang: en
---

This feature allows you to configure integration with a Vault server and use secrets in CI pipelines. Before getting started, you need to configure the Vault server and create the appropriate roles and policies.

## Vault configuration

1. Enable JWT authentication:

   ```bash
   vault auth enable jwt

   vault write auth/jwt/config \
     oidc_discovery_url="https://code.example.com" \
     bound_issuer="https://code.example.com" \
     default_role="gitlab-role"
   ```

1. Create a role:

   ```bash
   vault write auth/jwt/role/gitlab-role - <<EOF
   {
     "role_type": "jwt",
     "user_claim": "sub",
     "bound_audiences": ["vault"],
     "bound_claims": {
       "project_id": "23"
     },
     "policies": ["gitlab-policy"],
     "ttl": "1h"
   }
   EOF
   ```

   > Always use `bound_claims` to restrict access to the role. Otherwise, any JWT issued by the platform will be able to authenticate using this role.
  
1. Create a policy:

   ```bash
   vault policy write gitlab-policy - <<EOF
   path "kv/data/code/vault-demo" {
     capabilities = ["read"]
   }
   EOF
   ```

## CI configuration

### Environment variables

To work correctly with Vault in a CI/CD pipeline, you need to define the following environment variables:

- `VAULT_SERVER_URL` — **required**. The URL of the Vault server (e.g., `https://vault.example.com`).
- `VAULT_AUTH_ROLE` — *optional*. The name of the role in Vault. If not specified, the default role configured for the authentication method will be used.
- `VAULT_AUTH_PATH` — *optional*. Path to the authentication method in Vault. Defaults to `jwt`.
- `VAULT_NAMESPACE` — *optional*. Vault namespace, if a multi-level hierarchy is used.

### Using secrets in CI

To retrieve secrets from Vault, you can use the following job template:

```yaml
stages:
  - test
vault-login:
  stage: test
  image: ruby:3.2
  id_tokens:
    VAULT_ID_TOKEN:
      aud: vault
  secrets:
    DATABASE_PASSWORD:
      vault: code/vault-demo/DATABASE_PASSWORD@kv
      token: $VAULT_ID_TOKEN
  script: echo $DATABASE_PASSWORD
```

### Secret parameters

Example:

```yaml
DATABASE_PASSWORD:
  vault: code/vault-demo/DATABASE_PASSWORD@kv
  token: $VAULT_ID_TOKEN
  file: false
```

Parameter details:

1. `vault` (required) — the path to the secret in the string format `path/to/secret/KEY@ENGINE`, where:
   - `code/vault-demo/` — the path to the secret in Vault;
   - `DATABASE_PASSWORD` — the name of the field inside the secret;
   - `kv` — the mount point of the secret engine (default is `secret`).

By default, the `kv-v2` engine is used. If you need to use a different engine, you can specify it as an object instead of a string:

```yaml
DATABASE_PASSWORD:
  vault: 
    path: code/vault-demo
    field: DATABASE_PASSWORD
    engine:
      name: 'kv-v1'
      path: 'kv1'
  token: $VAULT_ID_TOKEN
  file: false
```

1. `token` (required) — a JWT token from the `id_tokens` section used to authenticate with Vault.

1. `file` (optional, defaults to `true`) — defines how the secret is provided:
   - `true` — the secret is saved to a temporary file;
   - `false` — the secret is passed as a string to an environment variable.

### JWT claims

The following fields are automatically included in the JWT token and can be used by Vault to validate access rights:

| Claim                   | Availability condition      | Description                                                              |
|-------------------------|-----------------------------|---------------------------------------------------------------------------|
| `jti`                   | always                      | Unique token identifier                                                   |
| `iss`                   | always                      | Token issuer (typically the Deckhouse Code URL)                          |
| `iat`                   | always                      | Token issue time (`Issued At`)                                           |
| `nbf`                   | always                      | Time before which the token is not valid                                 |
| `exp`                   | always                      | Token expiration time                                                    |
| `sub`                   | always                      | Token subject (usually the CI job ID)                                    |
| `namespace_id`          | always                      | ID of the namespace (group or user space)                                |
| `namespace_path`        | always                      | Path to the namespace (e.g., `groups/dev`)                               |
| `project_id`            | always                      | Project ID                                                               |
| `project_path`          | always                      | Path to the project                                                      |
| `user_id`               | always                      | User ID                                                                  |
| `user_login`            | always                      | User login                                                               |
| `user_email`            | always                      | User email                                                               |
| `pipeline_id`           | always                      | CI pipeline ID                                                           |
| `pipeline_source`       | always                      | Pipeline trigger source (push, schedule, merge request, etc.)           |
| `job_id`                | always                      | CI job ID                                                                |
| `ref`                   | always                      | Git reference (e.g., `main`, `v1.2`)                                           |
| `ref_type`              | always                      | Git reference type (`branch` or `tag`)                                             |
| `ref_path`              | always                      | Full Git reference path (e.g., `refs/heads/main`)                                  |
| `ref_protected`         | always                      | Indicates if the Git reference is protected                                        |
| `environment`           | if available                | Environment name (if used)                                               |
| `groups_direct`         | if available (<200 groups)  | Paths to groups the user is directly a member of                         |
| `environment_protected` | if available                | Indicates if the environment is protected                                |
| `deployment_tier`       | if available                | Environment type (`production`, `staging`, etc.)                         |
| `environment_action`    | if available                | Action being performed on the environment (e.g., `deploy`)               |
