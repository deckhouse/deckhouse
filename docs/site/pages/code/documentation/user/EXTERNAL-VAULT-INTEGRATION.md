---
title: "External Vault integration"
menuTitle: External Vault integration
force_searchable: true
description: Resolving secrets from an external Vault in CI
weight: 50
permalink: en/code/documentation/user/external-vault.html
lang: en
---

## Integration with External Vault

This feature allows you to set up integration with a Vault server and use secrets in CI pipelines.
To get started, you need to configure the Vault server and prepare the appropriate roles and policies.

### VAULT Setup

#### 1) Enabling JWT Authentication

  ```bash
  vault auth enable jwt

  vault write auth/jwt/config \
    oidc_discovery_url="https://code.example.com" \
    bound_issuer="https://code.example.com" \
    default_role="gitlab-role"
  ```

#### 2) Creating a Role

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

  > ⚠️ Important: Always use bound_claims to restrict access to the role.
  Otherwise, any JWT issued by the instance will be able to access Vault using this role.

#### 3) Configuring a Policy

  ```bash
  vault policy write gitlab-policy - <<EOF
  path "kv/data/code/vault-demo" {
    capabilities = ["read"]
  }
  EOF

  ```

### CI Configuration

#### Environment Variables

Set the following environment variables in CI/CD:

- `VAULT_SERVER_URL` - Required. Vault server URL, e.g., <https://vault.example.com>.
- `VAULT_AUTH_ROLE` - Optional. Role on the Vault server. If not set, the default role configured for the auth method will be used.
- `VAULT_AUTH_PATH` - Optional. Path to the authentication method. Default is `jwt`.
- `VAULT_NAMESPACE` - Optional. Vault namespace.

#### Using Secrets in CI

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

#### keys details

```yaml
DATABASE_PASSWORD:
  vault: code/vault-demo/DATABASE_PASSWORD@kv
  token: $VAULT_ID_TOKEN
  file: false
```

##### (Required)

A string in the format `code/vault-demo/DATABASE_PASSWORD@kv` where:

- `code/vault-demo/` – path to the secret
- `DATABASE_PASSWORD` – field name
- `kv` – secret engine mount point, default is secret

By default, the kv-v2 engine is used.
To use a different engine, you can provide an object instead of a string:

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

##### `token`  (Required)

Required parameter.
The JWT token from the id_tokens section used to authenticate with Vault.

##### `file` (опционально)

Default is true.
Defines whether the secret will be saved as a file or a string.

### Fields Included in JWT

The following fields are included in the JWT token:

| Field                    | When         | Description                                             |
|--------------------------|--------------|---------------------------------------------------------|
| `jti`                    | always       | Unique token identifier                                 |
| `iss`                    | always       | Issuer (Code URL)                                       |
| `iat`                    | always       | Issued at time                                          |
| `nbf`                    | always       | Not valid before                                        |
| `exp`                    | always       | Expiration time                                         |
| `sub`                    | always       | Subject (usually the job ID)                            |
| `namespace_id`           | always       | Group or user namespace ID                              |
| `namespace_path`         | always       | Group or user namespace path                            |
| `project_id`             | always       | Project ID                                              |
| `project_path`           | always       | Project path                                            |
| `user_id`                | always       | User ID                                                 |
| `user_login`            | always       | User login                                              |
| `user_email`            | always       | User email                                              |
| `pipeline_id`           | always       | Pipeline ID                                             |
| `pipeline_source`       | always       | Pipeline source                                         |
| `job_id`                | always       | CI job ID                                               |
| `ref`                   | always       | Git reference                                           |
| `ref_type`              | always       | Reference type (`branch` or `tag`)                      |
| `ref_path`              | always       | Full ref path (e.g., `refs/heads/main`)                |
| `ref_protected`         | always       | Indicates whether the ref is protected                 |
| `environment`           | if present   | Environment name                                        |
| `groups_direct`         | <200 groups  | Direct groups the user belongs to                      |
| `environment_protected` | if present   | Indicates if the environment is protected              |
| `deployment_tier`       | if present   | Environment type (production, staging, etc.)           |
| `environment_action`    | if present   | Specified action on the environment                    |

references  :
- <https://docs.gitlab.com/ci/secrets/hashicorp_vault/>
