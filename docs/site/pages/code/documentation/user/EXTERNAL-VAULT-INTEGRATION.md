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

## Quick start

This section provides an example of the minimum required configuration
for integrating HashiCorp Vault with Deckhouse Code and verifying that a CI job can retrieve secrets from Vault.

{% alert level="warning" %}
This example is provided for demonstration purposes only.
It does not reflect security best practices and uses a simplified configuration
to allow you to quickly verify that the integration works.
{% endalert %}

### Step 1. Set environment variables

Set the environment variables for Vault and Deckhouse Code.
Some parameters can be left unchanged, but `VAULT_ADDR`, `VAULT_TOKEN`, `CODE_URL`,
and `PROJECT_PATH` must be set manually.

```bash
export VAULT_ADDR="https://vault.example.com"
export VAULT_TOKEN="<your-token>"

# Deckhouse Code URL.
export CODE_URL="https://code.example.com"

# Vault role and policy names.
export VAULT_ROLE="code-role"
export VAULT_POLICY="code-policy"

# Secret path and data.
export VAULT_SECRET_PATH="code/vault-demo"
export VAULT_SECRET_FIELD="DATABASE_PASSWORD"
export VAULT_SECRET_VALUE="super-secret-password"

# Value of the project_path claim that Vault will validate.

export PROJECT_PATH="root/my-pr"
```

### Step 2. Enable the JWT authentication method

Enable the JWT authentication method in Vault.
Without it, Vault will not be able to accept ID tokens that Deckhouse Code passes to CI jobs.

```bash
curl \
  -H "X-Vault-Token: $VAULT_TOKEN" \
  -X POST "$VAULT_ADDR/v1/sys/auth/jwt" \
  -d '{"type":"jwt"}'
```

### Step 3. Configure JWT and OIDC

The following request:

- Sets the OIDC discovery URL (`$CODE_URL`).
- Specifies the expected issuer.
- Defines the *default role* that Vault issues during authentication.

```bash
curl \
  -H "X-Vault-Token: $VAULT_TOKEN" \
  -X POST "$VAULT_ADDR/v1/auth/jwt/config" \
  --data @- <<EOF
{
  "oidc_discovery_url": "$CODE_URL",
  "bound_issuer": "$CODE_URL",
  "default_role": "$VAULT_ROLE"
}
EOF
```

### Step 4. Mount the KV v2 secret engine

KV v2 is the most commonly used Vault secret engine.
The following request enables it at the `/kv` path:

```bash
curl \
  -H "X-Vault-Token: $VAULT_TOKEN" \
  -X POST "$VAULT_ADDR/v1/sys/mounts/kv" \
  -d '{"type": "kv-v2"}'
```

### Step 5. Create a test secret

Create a secret that will later be read by a CI job.
The secret is stored at the `code/vault-demo` path and contains a single field.

```bash
curl \
  -H "X-Vault-Token: $VAULT_TOKEN" \
  -X POST "$VAULT_ADDR/v1/kv/data/$VAULT_SECRET_PATH" \
  --data @- <<EOF
{
  "data": {
    "$VAULT_SECRET_FIELD": "$VAULT_SECRET_VALUE"
  }
}
EOF
```

### Step 6. Create an ACL policy

The policy defines which paths in Vault can be accessed.
In the following example, the policy grants read-only access to the specified secret.

```bash
curl \
  -H "X-Vault-Token: $VAULT_TOKEN" \
  -X PUT "$VAULT_ADDR/v1/sys/policies/acl/$VAULT_POLICY" \
  --data @- <<EOF
{
  "policy": "path \"kv/data/$VAULT_SECRET_PATH\" { capabilities = [\"read\"] }"
}
EOF
```

### Step 7. Create a Vault role

The role defines:

- Authentication type
- Required claims in the token (`project_path`)
- Policies granted to the authenticated subject
- Allowed audiences (`aud`)
- Token TTL

Deckhouse Code will issue an ID token with `aud=vault`,
and Vault will verify that the `project_path` value matches the configured one.

```bash
curl \
  -H "X-Vault-Token: $VAULT_TOKEN" \
  -X POST "$VAULT_ADDR/v1/auth/jwt/role/$VAULT_ROLE" \
  --data @- <<EOF
{
  "role_type":   "jwt",
  "user_claim":  "sub",
  "bound_audiences": ["vault"],

  "bound_claims": {
    "project_path": "$PROJECT_PATH"
  },

  "policies": ["$VAULT_POLICY"],
  "ttl": "1h"
}
EOF
```

At this point, the Vault configuration is complete.

### Testing the integration in Deckhouse Code

1. Open the project specified in `PROJECT_PATH`.
   The project CI token must match the `project_path` claim.
   Otherwise, Vault will deny access to secrets.

1. In the project CI/CD settings, add the `VAULT_SERVER_URL` variable with the value of `$VAULT_ADDR` used earlier.
   This variable tells Deckhouse Code where to send Vault API requests.

1. Create the `.gitlab-ci.yml` file.
   The file runs a test CI job that:

   - Obtains an ID token with `aud=vault`.
   - Passes it to Vault.
   - Retrieves a secret from KV.
   - Outputs the secret value.

   ```yml
   stages:
     - test
   
   vault-demo:
     stage: test
     image: alpine
     id_tokens:
       VAULT_ID_TOKEN:
         aud: vault
     secrets:
       DATABASE_PASSWORD:
         vault: code/vault-demo/DATABASE_PASSWORD@kv
         token: $VAULT_ID_TOKEN
         file: false
     script:
       - echo "Raw value (masked by GitLab):"
       - echo "$DATABASE_PASSWORD"
   
       - echo
       - echo "Value with spaces (not masked):"
       - printf '%s\n' "$DATABASE_PASSWORD" | sed 's/./& /g'
   ```

### Result

If the integration is configured correctly:

- The CI job successfully obtains an ID token.
- Vault validates the `project_path` value and grants access.
- The secret is retrieved and printed to the log.

In the job output, you will see the value of the `DATABASE_PASSWORD` field loaded directly from Vault.

## Deckhouse Stronghold integration

[Deckhouse Stronghold](/products/stronghold/) is an enterprise secrets management solution built on top of HashiCorp Vault. Integrating Stronghold with Deckhouse Code allows you to securely use secrets in CI/CD pipelines without storing them in project variables.

### Interaction flow

![Stronghold and Deckhouse Code integration](/images/code/stronghold_code_en.png)

Integration benefits:

- **Centralized secrets management** — all secrets are stored in one place with access auditing.
- **Automatic rotation** — secrets can be automatically rotated without modifying pipelines.
- **Granular access control** — access to secrets is restricted based on project, branch, environment, and other parameters.
- **No secrets in the repository** — secrets never end up in the code or CI variables.

### Usage examples

This section provides common scenarios for using Vault secrets in CI/CD pipelines.

#### Deploying an application with database credentials

Example of retrieving PostgreSQL credentials during deployment:

```yaml
stages:
  - deploy

deploy-production:
  stage: deploy
  image: alpine/k8s:1.28.0
  environment:
    name: production
  id_tokens:
    VAULT_ID_TOKEN:
      aud: vault
  secrets:
    DB_HOST:
      vault: apps/myapp/production/DB_HOST@kv
      token: $VAULT_ID_TOKEN
      file: false
    DB_USER:
      vault: apps/myapp/production/DB_USER@kv
      token: $VAULT_ID_TOKEN
      file: false
    DB_PASSWORD:
      vault: apps/myapp/production/DB_PASSWORD@kv
      token: $VAULT_ID_TOKEN
      file: false
  script:
    - kubectl create secret generic db-credentials \
        --from-literal=host=$DB_HOST \
        --from-literal=username=$DB_USER \
        --from-literal=password=$DB_PASSWORD \
        --dry-run=client -o yaml | kubectl apply -f -
    - kubectl rollout restart deployment/myapp
  rules:
    - if: $CI_COMMIT_BRANCH == "main"
```

#### Using API keys for external services

Example of integration with external APIs (Slack, Telegram, S3):

```yaml
stages:
  - notify
  - backup

notify-slack:
  stage: notify
  image: curlimages/curl:latest
  id_tokens:
    VAULT_ID_TOKEN:
      aud: vault
  secrets:
    SLACK_WEBHOOK_URL:
      vault: integrations/slack/WEBHOOK_URL@kv
      token: $VAULT_ID_TOKEN
      file: false
  script:
    - |
      curl -X POST "$SLACK_WEBHOOK_URL" \
        -H "Content-Type: application/json" \
        -d "{\"text\": \"Deployment completed for $CI_PROJECT_NAME\"}"

backup-to-s3:
  stage: backup
  image: amazon/aws-cli:latest
  id_tokens:
    VAULT_ID_TOKEN:
      aud: vault
  secrets:
    AWS_ACCESS_KEY_ID:
      vault: cloud/aws/backup-user/ACCESS_KEY_ID@kv
      token: $VAULT_ID_TOKEN
      file: false
    AWS_SECRET_ACCESS_KEY:
      vault: cloud/aws/backup-user/SECRET_ACCESS_KEY@kv
      token: $VAULT_ID_TOKEN
      file: false
  script:
    - aws s3 sync ./artifacts s3://my-backup-bucket/$CI_PROJECT_NAME/$CI_COMMIT_SHA/
```

#### Signing Docker images with Cosign

Example of using a private key from Vault to sign images:

```yaml
stages:
  - build
  - sign

build-image:
  stage: build
  image: docker:24.0.5
  services:
    - docker:24.0.5-dind
  script:
    - docker build -t $CI_REGISTRY_IMAGE:$CI_COMMIT_SHA .
    - docker push $CI_REGISTRY_IMAGE:$CI_COMMIT_SHA

sign-image:
  stage: sign
  image: bitnami/cosign:latest
  id_tokens:
    VAULT_ID_TOKEN:
      aud: vault
  secrets:
    COSIGN_PRIVATE_KEY:
      vault: signing/cosign/PRIVATE_KEY@kv
      token: $VAULT_ID_TOKEN
      file: true
  script:
    - cosign sign --key $COSIGN_PRIVATE_KEY $CI_REGISTRY_IMAGE:$CI_COMMIT_SHA
```

#### Branch-based access control

Example of configuring different secrets for `develop` and `main` branches using bound claims on branch (`ref`):

```yaml
stages:
  - deploy

.deploy-template:
  image: alpine/k8s:1.28.0
  id_tokens:
    VAULT_ID_TOKEN:
      aud: vault
  script:
    - echo "Deploying from branch $CI_COMMIT_BRANCH with database $DB_HOST"
    - kubectl set env deployment/myapp DB_HOST=$DB_HOST DB_PASSWORD=$DB_PASSWORD

deploy-develop:
  extends: .deploy-template
  stage: deploy
  variables:
    VAULT_AUTH_ROLE: myapp-develop
  secrets:
    DB_HOST:
      vault: apps/myapp/develop/DB_HOST@kv
      token: $VAULT_ID_TOKEN
      file: false
    DB_PASSWORD:
      vault: apps/myapp/develop/DB_PASSWORD@kv
      token: $VAULT_ID_TOKEN
      file: false
  rules:
    - if: $CI_COMMIT_BRANCH == "develop"

deploy-main:
  extends: .deploy-template
  stage: deploy
  variables:
    VAULT_AUTH_ROLE: myapp-main
  secrets:
    DB_HOST:
      vault: apps/myapp/main/DB_HOST@kv
      token: $VAULT_ID_TOKEN
      file: false
    DB_PASSWORD:
      vault: apps/myapp/main/DB_PASSWORD@kv
      token: $VAULT_ID_TOKEN
      file: false
  rules:
    - if: $CI_COMMIT_BRANCH == "main"
```

Example of configuring a Vault role with bound claims based on branch for access control:

```bash
# Role for `develop` — access only from `develop` branch.
vault write auth/jwt/role/myapp-develop - <<EOF
{
  "role_type": "jwt",
  "user_claim": "sub",
  "bound_audiences": ["vault"],
  "bound_claims": {
    "project_path": "myteam/myapp",
    "ref": "develop",
    "ref_type": "branch"
  },
  "policies": ["myapp-develop-policy"],
  "ttl": "1h"
}
EOF

# Role for `main` — access only from `main` branch (protected).
vault write auth/jwt/role/myapp-main - <<EOF
{
  "role_type": "jwt",
  "user_claim": "sub",
  "bound_audiences": ["vault"],
  "bound_claims": {
    "project_path": "myteam/myapp",
    "ref": "main",
    "ref_type": "branch",
    "ref_protected": "true"
  },
  "policies": ["myapp-main-policy"],
  "ttl": "1h"
}
EOF
```
