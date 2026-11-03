---
title: "Access for CI/CD"
permalink: en/admin/configuration/access/authorization/ci_cd.html
description: "Configure CI/CD access to Kubernetes API in Deckhouse: ServiceAccount, Basic Auth, and Token Exchange."
---

The following methods are available for authenticating CI/CD pipelines in the Kubernetes API:
- [ServiceAccount](#serviceaccount) — Kubernetes ServiceAccount token.
- [Basic Auth](#basic-auth) — username and password via IdP.
- [Token Exchange](#token-exchange) — exchange IdP token for Dex token.

---

## ServiceAccount

The ServiceAccount token is used directly for API authentication. No external IdP is required. For ServiceAccount, you can create [long-lived tokens](#create-serviceaccount-and-a-long-lived-token) (without TTL, with the possibility of revocation or rotation) and [short-lived tokens](#short-lived-tokens-tokenrequest).

When multiple pipelines share a single ServiceAccount, audit logs will not contain information about specific pipelines.

### Prerequisites

To configure token-based authentication for ServiceAccount, the following requirements must be met:

- Cluster access with permissions to create ServiceAccounts and Secrets.
- For external access: [apiserver.publishAPI](/modules/control-plane-manager/configuration.html#parameters-apiserver-publishapi) of the `control-plane-manager` module, or direct API access via VPN.

### Create ServiceAccount and a long-lived token

To create a ServiceAccount and token, use the command:

```shell
d8 k create ns ci-deploy || true

cat <<EOF | d8 k apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: gitlab-runner-deploy
  namespace: ci-deploy
---
apiVersion: v1
kind: Secret
metadata:
  name: gitlab-runner-deploy-token
  namespace: ci-deploy
  annotations:
    kubernetes.io/service-account.name: gitlab-runner-deploy
type: kubernetes.io/service-account-token
EOF
```

{% alert level="info" %}
The `kubernetes.io/service-account-token` Secret type is a legacy approach. The recommended method is the TokenRequest API (`d8 k create token ...`).
{% endalert %}

### Grant permissions

For details on granting permissions, see [Granting permissions to users and service accounts](granting.html).

Assign the necessary permissions to the ServiceAccount.

For the basic role model, use [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule):

```shell
cat <<EOF | d8 k apply -f -
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: gitlab-runner-access
spec:
  subjects:
  - kind: ServiceAccount
    name: gitlab-runner-deploy
    namespace: ci-deploy
  accessLevel: Admin
  portForwarding: true
EOF
```

Available levels: `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`, `ClusterAdmin`, `SuperAdmin`.

For the granular role model, grant the `d8:project:admin` role in all projects with a [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding). To grant access to a single namespace only, create a RoleBinding to the `d8:namespace:admin` role in it instead:

```shell
cat <<EOF | d8 k apply -f -
apiVersion: deckhouse.io/v1alpha3
kind: ClusterProjectRoleBinding
metadata:
  name: gitlab-runner-access
spec:
  subjects:
  - kind: ServiceAccount
    name: gitlab-runner-deploy
    namespace: ci-deploy
  roleRef:
    kind: ClusterRole
    name: d8:project:admin
EOF
```

### Get API URL and CA certificate

When using publishAPI:

{% tabs api_publish_type %}
{% tab "When the API server is published via Ingress" %}

When the API server is published via Ingress, use the following commands:

```shell
API_HOST=$(d8 k -n kube-system get ingress kubernetes-api -o jsonpath='{.spec.rules[0].host}')
echo "API endpoint: https://${API_HOST}"
```

{% endtab %}
{% tab "When the API server is published via Gateway API (`alb`)" %}

When the API server is published via Gateway API (the [`alb`](/modules/alb/) module), use the following commands:

```shell
API_HOST=$(d8 k -n kube-system get httproute kubernetes-api -o jsonpath='{.spec.hostnames[0]}')
echo "API endpoint: https://${API_HOST}"
```

{% endtab %}
{% endtabs %}

{% alert level="info" %}
If the API certificate is signed by a public CA (Let's Encrypt), the `--certificate-authority` parameter is not required.
{% endalert %}

For a private CA (self-signed, the default mode — shared between both publication methods):

```shell
d8 k -n kube-system get secret kubernetes-api-ca-key-pair -o jsonpath='{.data.tls\.crt}' | base64 -d > /tmp/ca.crt
```

### Create kubeconfig

Example of generating kubeconfig for a previously created ServiceAccount:

```shell
export CLUSTER_NAME=my-cluster
export USER_NAME=gitlab-runner-deploy
export CONTEXT_NAME=${CLUSTER_NAME}-${USER_NAME}
export FILE_NAME=kube.config

# For public CA, remove --certificate-authority and --embed-certs
d8 k config set-cluster $CLUSTER_NAME \
  --server=https://${API_HOST} \
  --certificate-authority=/tmp/ca.crt \
  --embed-certs=true \
  --kubeconfig=$FILE_NAME

d8 k config set-credentials $USER_NAME \
  --token=$(d8 k -n ci-deploy get secret gitlab-runner-deploy-token -o jsonpath='{.data.token}' | base64 -d) \
  --kubeconfig=$FILE_NAME

d8 k config set-context $CONTEXT_NAME \
  --cluster=$CLUSTER_NAME --user=$USER_NAME \
  --kubeconfig=$FILE_NAME

d8 k config use-context $CONTEXT_NAME --kubeconfig=$FILE_NAME
```

### Short-lived tokens (TokenRequest)

The TokenRequest API allows creating tokens with limited lifetime. When creating such a token, specify the name of the ServiceAccount that needs to be granted access in the command (in the example — `gitlab-runner-deploy`):

```shell
d8 k create token gitlab-runner-deploy -n ci-deploy --duration=1h
```

{% alert level="warning" %}
The `d8 k create token` command itself requires access to the Kubernetes API. If the CI job has no initial cluster access, it cannot execute TokenRequest from within the job.
{% endalert %}

#### Usage patterns in CI

**Bootstrap token with minimal permissions.** CI stores a long-lived kubeconfig/token that only has permission to create TokenRequest for a specific ServiceAccount. At the start of the job, the bootstrap token issues a short-lived token used for deployment.

**External token broker.** An operator, secret manager (Vault, External Secrets), or external service periodically issues short-lived tokens and places them in CI variables or secrets. The CI job only consumes the ready token.

{% alert level="info" %}
The bootstrap token should have minimal permissions: only `create` on the `serviceaccounts/token` resource for a specific ServiceAccount/namespace. Do not use cluster-admin. Limit the TTL of issued tokens and promptly revoke bootstrap access if compromised.
{% endalert %}

Usage without kubeconfig file:

```shell
export KUBE_SERVER="https://${API_HOST}"
export KUBE_TOKEN=$(d8 k create token gitlab-runner-deploy -n ci-deploy --duration=1h)
d8 k --server=$KUBE_SERVER --token=$KUBE_TOKEN get ns
```

---

## Basic Auth

Authentication via username and password through IdP (LDAP, OIDC).

{% alert level="warning" %}
The password is sent to DP and validated through basic-auth-proxy/Dex.
{% endalert %}

{% alert level="warning" %}
Only one DexProvider in the cluster can have `enableBasicAuth: true`.
{% endalert %}

### Prerequisites

The following requirements must be met to configure Basic Auth:

- [apiserver.publishAPI](/modules/control-plane-manager/configuration.html#parameters-apiserver-publishapi) of the `control-plane-manager` module enabled.
- [DexProvider](/modules/user-authn/cr.html#dexprovider) configured for IdP.

### Enable

Add `enableBasicAuth: true` to DexProvider. For DexProvider configuration examples, see [user-authn module documentation](/modules/user-authn/usage.html).

### Get API endpoint

To get the API endpoint, use the command:

```shell
API_HOST=$(d8 k -n d8-user-authn get ingress kubernetes-api -o jsonpath='{.spec.rules[0].host}')
echo "https://${API_HOST}"
```

### Verification

To check access, use the command:

```shell
curl -q -u "$K8S_USER:$K8S_PASSWORD" "https://${API_HOST}/version"
```

Possible errors:

- `401` — invalid credentials or Basic Auth not enabled.
- `403` — authentication succeeded but RBAC denies access.

### Configure kubeconfig

Example of kubeconfig configuration for Basic Auth:

{% raw %}

```yaml
apiVersion: v1
kind: Config
clusters:
- name: my-cluster
  cluster:
    server: https://<API_HOST>
users:
- name: basic-auth-user
  user:
    username: "<USERNAME>"
    password: "<PASSWORD>"
contexts:
- name: default
  context:
    cluster: my-cluster
    user: basic-auth-user
current-context: default
```

{% endraw %}

### Use in GitLab CI

Example of using Basic Auth in GitLab CI:

{% raw %}

```yaml
deploy:
  script:
    - d8 k --server="$K8S_SERVER" --username="$K8S_USER" --password="$K8S_PASSWORD" get ns
```

{% endraw %}

Variables `K8S_SERVER`, `K8S_USER`, `K8S_PASSWORD` are set in the [project](../../../../glossary.html#project) CI/CD settings.

---

## Token Exchange

The pipeline gets a token from its CI system, exchanges it at Dex for a token with `aud=kubernetes`, and uses it to access the API.

{% alert level="info" %}
Recommended for GitLab CI and GitHub Actions.
{% endalert %}

DP/Dex receives no password. Dex trusts the CI system as a [TokenExchangeProvider](/modules/user-authn/cr.html#tokenexchangeprovider). It checks the signature, the issuer, the expiry and the audience of the token and the required claims, and names the pipeline in the cluster by a claim of the token.

### Prerequisites

The following requirements must be met to configure Token Exchange:

- [apiserver.publishAPI](/modules/control-plane-manager/configuration.html#parameters-apiserver-publishapi) of the `control-plane-manager` module enabled.
- The CI system issues signed JWT ID tokens (GitLab CI `id_tokens`, GitHub Actions OIDC) and publishes its discovery document, or you have its public keys.

### Create DexClient

To create DexClient, use the command:

```shell
cat <<EOF | d8 k apply -f -
apiVersion: deckhouse.io/v1
kind: DexClient
metadata:
  name: ci-token-exchange
  namespace: d8-user-authn
  annotations:
    dexclient.deckhouse.io/allow-access-to-kubernetes: "true"
spec: {}
EOF
```

The annotation `dexclient.deckhouse.io/allow-access-to-kubernetes` allows the client to request tokens with `aud=kubernetes`.

{% alert level="warning" %}
This grants cluster-wide access to the Kubernetes API. For this reason, adding the annotation or changing it to `"true"` requires the `create` permission on the `dexclients/allow-access-to-kubernetes` subresource of the `deckhouse.io` API group in the DexClient namespace. In the basic role model, this permission is granted at the `ClusterAdmin` level and above. In the granular model, it is granted by the `d8:subsystem:iam:manager` role and the roles that include it, such as `d8:system:manager`. A namespace-level role that permits creating a DexClient is not enough: the admission policy rejects the request with a message naming the annotation.

The addition of the annotation is restricted regardless of the specified value, including `"false"`. This is required to ensure compatibility with previous DP versions where access has been granted when the annotation was present, regardless of its value.

This restriction is not applicable to an object, which already has the annotation assigned. Deletion and recreation of DexClient is treated as another addition of the annotation, so a GitOps controller that recreates the object instead of patching it requires the same `create` permission on the `dexclients/allow-access-to-kubernetes` subresource.
{% endalert %}

Get client secret:

```shell
d8 k -n d8-user-authn get secret dex-client-ci-token-exchange -o jsonpath='{.data.clientSecret}' | base64 -d
```

**client_id** format: `dex-client-ci-token-exchange@d8-user-authn`.

### Create TokenExchangeProvider

TokenExchangeProvider tells Dex which tokens to accept and which DexClient may exchange them. Example for GitLab CI:

```shell
cat <<EOF | d8 k apply -f -
apiVersion: deckhouse.io/v1alpha1
kind: TokenExchangeProvider
metadata:
  name: gitlab-ci
spec:
  issuer: https://gitlab.example.com
  keys:
    discovery: {}
  audiences:
  - exact: https://<DEX_HOST>/
  claimValidation:
  - claim: namespace_path
    equals: platform
  - claim: ref_protected
    equals: "true"
  identity:
    username:
      claim: sub
      prefix: "gitlab-ci:"
  clients:
  - dexClient:
      name: ci-token-exchange
      namespace: d8-user-authn
EOF
```

Parameters:

- `issuer`: URL of the GitLab instance, as in the `iss` claim of its ID tokens.
- `audiences`: Value of `aud` that the job sets in `id_tokens`. A token issued for another service is refused.
- `claimValidation`: Claims that a token must carry. In the example, only protected branches of projects in the `platform` group are accepted.
- `identity.username`: User name in the cluster, for example `gitlab-ci:project_path:platform/app:ref_type:branch:ref:main` for the `main` branch of `platform/app`.

Get the connector ID for the exchange request:

```shell
CONNECTOR_ID=$(d8 k get tokenexchangeprovider gitlab-ci -o jsonpath='{.status.connectorID}')
```

Store it as a CI variable. The ID changes if the object is deleted and created again, or if the provider gets its connector back after a `PrefixConflict`. In these cases, update the variable, otherwise Dex answers with 400 `Requested connector does not exist`.

### Get Dex and API URLs

To get the Dex URL and API, use the following commands:

```shell
DEX_HOST=$(d8 k -n d8-user-authn get ingress dex -o jsonpath='{.spec.rules[0].host}')
API_HOST=$(d8 k -n d8-user-authn get ingress kubernetes-api -o jsonpath='{.spec.rules[0].host}')
```

### Grant RBAC

DP configures kube-apiserver to validate Dex tokens. The user name is the one TokenExchangeProvider sets in `identity.email` if it is set, and in `identity.username` otherwise; groups are taken from `identity.groups`.

For details on granting permissions, see [Granting permissions to users and service accounts](granting.html).

For the basic role model, use [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule):

```shell
cat <<EOF | d8 k apply -f -
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: ci-deployer
spec:
  subjects:
  - kind: User
    name: "gitlab-ci:project_path:platform/app:ref_type:branch:ref:main"
  accessLevel: Admin
EOF
```

For the granular role model, grant the `d8:project:admin` role in all projects with a [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding). To grant access to a single namespace only, create a RoleBinding to the `d8:namespace:admin` role in it instead:

```shell
cat <<EOF | d8 k apply -f -
apiVersion: deckhouse.io/v1alpha3
kind: ClusterProjectRoleBinding
metadata:
  name: ci-deployer
spec:
  subjects:
  - kind: User
    name: "gitlab-ci:project_path:platform/app:ref_type:branch:ref:main"
  roleRef:
    kind: ClusterRole
    name: d8:project:admin
EOF
```

### Token exchange request

Example of a token exchange request:

```shell
RESPONSE=$(curl -q -s -X POST "https://${DEX_HOST}/token" \
  -u "${DEX_CLIENT_ID}:${DEX_CLIENT_SECRET}" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=urn:ietf:params:oauth:grant-type:token-exchange" \
  -d "subject_token=${CI_TOKEN}" \
  -d "subject_token_type=urn:ietf:params:oauth:token-type:id_token" \
  -d "connector_id=${CONNECTOR_ID}" \
  -d "scope=openid email groups federated:id audience:server:client_id:kubernetes" \
  -d "requested_token_type=urn:ietf:params:oauth:token-type:id_token")

DEX_TOKEN=$(echo "$RESPONSE" | jq -r '.access_token')
```

Parameters:

- `subject_token`: ID token of the CI system. Access tokens are not accepted.
- `connector_id`: `status.connectorID` of TokenExchangeProvider.
- `scope`: Must include the following values:
  - `openid`: Required for an ID token.
  - `email`: kube-apiserver takes the user name from the `email` claim. Without it, the API answers 401.
  - `audience:server:client_id:kubernetes`: The token is issued for kube-apiserver.
  - `groups`: Required if RBAC grants rights to groups.
  - `federated:id`: Shows the TokenExchangeProvider of a token in `d8 k auth whoami` (the `user-authn.deckhouse.io/dex-provider` extra).

The Dex token lives as long as `idTokenTTL` of the `user-authn` module (10 minutes by default). Request a new one for a longer job.

### GitLab CI

Example of a token request for GitLab CI:

{% raw %}

```yaml
deploy:
  id_tokens:
    GITLAB_OIDC_TOKEN:
      aud: https://<DEX_HOST>/
  script:
    - |
      RESPONSE=$(curl -q -s -X POST "https://${DEX_HOST}/token" \
        -u "${DEX_CLIENT_ID}:${DEX_CLIENT_SECRET}" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "grant_type=urn:ietf:params:oauth:grant-type:token-exchange" \
        -d "subject_token=${GITLAB_OIDC_TOKEN}" \
        -d "subject_token_type=urn:ietf:params:oauth:token-type:id_token" \
        -d "connector_id=${CONNECTOR_ID}" \
        -d "scope=openid email groups federated:id audience:server:client_id:kubernetes" \
        -d "requested_token_type=urn:ietf:params:oauth:token-type:id_token")
      DEX_TOKEN=$(echo "$RESPONSE" | jq -r '.access_token')
      d8 k --server="${K8S_SERVER}" --token="${DEX_TOKEN}" auth whoami
```

{% endraw %}

### GitHub Actions

For GitHub Actions, set the following in TokenExchangeProvider:

- `issuer`: `https://token.actions.githubusercontent.com`.
- `audiences`: Value that the workflow passes as `audience`.
- `claimValidation`: `repository_owner_id` and, if needed, `ref`. GitHub recommends checking the IDs, which never change. The name of a renamed or deleted organization can be taken by someone else.
- `identity.username`: `claim: sub` with a prefix, for example `github:`. The user name becomes `github:repo:my-org/app:ref:refs/heads/main`.

To request the token, add the `id-token: write` permission to the workflow:

{% raw %}

```yaml
permissions:
  id-token: write
steps:
  - run: |
      CI_TOKEN=$(curl -sS -H "Authorization: bearer ${ACTIONS_ID_TOKEN_REQUEST_TOKEN}" \
        "${ACTIONS_ID_TOKEN_REQUEST_URL}&audience=https://<DEX_HOST>/" | jq -r '.value')
```

{% endraw %}

Exchange `CI_TOKEN` as in the [token exchange request](#token-exchange-request).

### Migration from DexProvider

Earlier, a token was exchanged through a DexProvider of OIDC type, and the exchange request named it in `connector_id`. This method is deprecated and off by default. A DexProvider exchanges tokens only with `spec.tokenExchange.enabled: true`. The module set this value during the upgrade in clusters that had a DexClient with the `dexclient.deckhouse.io/allow-access-to-kubernetes` annotation. The parameter can only be turned off. It will be removed after the next LTS release, together with this method. While a cluster exchanges tokens through a DexProvider, the `D8UserAuthnDexProviderTokenExchangeDeprecated` alert fires.

To migrate, follow these steps:

1. Create a TokenExchangeProvider with the `issuer` of the DexProvider and the audience your CI tokens carry.
1. Set `identity.username` to the claim the DexProvider took the user name from, for example `claim: user_email` for GitLab CI, with a prefix such as `gitlab-ci:`; give the groups a prefix too. The prefix is required. It sets the subjects of the provider apart from people and from the subjects of other providers.
1. Keep the checks of the DexProvider:
   - A DexProvider accepted only a token with `email_verified: true`, unless `insecureSkipEmailVerified` was set. If the user name comes from an email, add `claimValidation: [{claim: email_verified, equals: "true"}]`.
   - Move `allowedEmails`, `allowedEmailDomains` and `allowedGroups` to `allowedIdentities`. Dex applies them to the names with the prefixes added, such as `gitlab-ci:deployer@example.com`. Add the prefixes of `identity` to `allowedEmails` and `allowedGroups`. Move `allowedEmailDomains` as they are, because the domain is the part after the last `@`.
   - `allowedGroups` needs `identity.groups`. Without groups, every exchange is refused.
1. Before the switch, add the prefixed subjects to the ClusterAuthorizationRule, AuthorizationRule and RoleBinding objects that grant rights to the old names, for example `gitlab-ci:deployer@example.com` next to `deployer@example.com`.
1. In the exchange request, replace `connector_id` with `status.connectorID` of the new object.
1. After the switch, remove the old subjects from these objects.
1. Turn off token exchange through the DexProvider by setting `spec.tokenExchange.enabled: false`. It cannot be turned on again.

A TokenExchangeProvider accepts JWT ID tokens only. A pipeline that exchanged an access token, for example one of Keycloak, has to request an ID token of the issuer instead.

### Diagnostics

{% alert level="info" %}
Use `d8 k auth whoami` to verify which identity is being used.
{% endalert %}

**`--token` flag may not work** — kubeconfig with `auth-provider`, `exec`, or `tokenFile` overrides the token from the flag. Use `-v=8` for debugging:

```shell
d8 k --server="https://${API_HOST}" --token="${DEX_TOKEN}" auth whoami -v=8
```

The output shows which Authorization header is sent. For reliable token usage, use a minimal kubeconfig with only `server` and `token`, without `auth-provider`/`exec`.

**TokenExchangeProvider is not ready** — check the `Ready` condition of the object:

```shell
d8 k get tokenexchangeprovider gitlab-ci -o jsonpath='{.status.conditions}'
```

**Dex 400** — one of the following:

- An unknown `subject_token_type`, or a wrong `connector_id` (`Requested connector does not exist`).
- The DexClient is not listed in `spec.clients` (`Connector not allowed for this client.`).
- `invalid_scope`: The DexClient has no `dexclient.deckhouse.io/allow-access-to-kubernetes` annotation, so it may not request `audience:server:client_id:kubernetes`. If the annotation is gone, check whether the admission-controller rejected the request that was supposed to add it, or whether a GitOps controller recreated the object without it. Adding the annotation requires the `create` permission on the `dexclients/allow-access-to-kubernetes` subresource of the `deckhouse.io` API group in the DexClient namespace.
- `unauthorized_client`: The client is a kubeconfig client or `kubernetes`, which may not exchange tokens.

**Dex 401** — invalid client credentials, or the token is refused: an access token instead of an ID token, wrong signature or issuer, a `kid` of no pinned key, expired or valid for longer than `subjectTokenMaxLifetime`, another audience, a required claim does not match, or the names with the prefixes added are not in `allowedIdentities` (`User is not allowed.`). The Dex log names the reason.

**API 401** — token validation failed. Check:

- `scope` contains `email` and `audience:server:client_id:kubernetes`.
- Time synchronization between CI runner and cluster.

**API 403** — authentication succeeded but RBAC does not allow access for the user or group from the token, or TokenExchangeProvider was deleted or lost its connector (`PrefixConflict`). In this case, every token issued through it is refused if the `enableMultiTenancy` parameter of the `user-authz` module is enabled and the cluster runs Kubernetes 1.34 or later.

Decode token to check claims:

```shell
echo "${DEX_TOKEN}" | cut -d. -f2 | base64 -d 2>/dev/null | jq .
```
