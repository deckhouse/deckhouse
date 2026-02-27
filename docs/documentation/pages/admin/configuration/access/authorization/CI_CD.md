---
title: "Access for CI/CD"
permalink: en/admin/configuration/access/authorization/ci_cd.html
description: "Configure CI/CD access to Kubernetes API in Deckhouse: ServiceAccount, Basic Auth, and Token Exchange."
---

Three methods are available for authenticating CI/CD pipelines to the Kubernetes API:
- [ServiceAccount](#serviceaccount) — Kubernetes ServiceAccount token.
- [Basic Auth](#basic-auth) — username and password via IdP.
- [Token Exchange](#token-exchange) — exchange IdP token for Dex token.

---

## ServiceAccount

The ServiceAccount token is used directly for API authentication. No external IdP is required.

When multiple pipelines share a single ServiceAccount, audit logs will not contain information about specific pipelines.

### Prerequisites

- Cluster access with permissions to create ServiceAccounts and Secrets.
- For external access: [publishAPI](/modules/user-authn/configuration.html#parameters-publishapi) or direct API access via VPN.

### Create ServiceAccount and token

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

[ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule):

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

Experimental role model — [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/):

```shell
cat <<EOF | d8 k apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: gitlab-runner-access
subjects:
- kind: ServiceAccount
  name: gitlab-runner-deploy
  namespace: ci-deploy
roleRef:
  kind: ClusterRole
  name: d8:manage:all:manager
  apiGroup: rbac.authorization.k8s.io
EOF
```

### Get API URL and CA certificate

When using publishAPI:

```shell
API_HOST=$(d8 k -n d8-user-authn get ingress kubernetes-api -o jsonpath='{.spec.rules[0].host}')
echo "API endpoint: https://${API_HOST}"
```

{% alert level="info" %}
If the API certificate is signed by a public CA (Let's Encrypt), the `--certificate-authority` parameter is not required.
{% endalert %}

For private CA:

```shell
d8 k -n d8-user-authn get secret kubernetes-api-ca-key-pair -o jsonpath='{.data.ca\.crt}' | base64 -d > /tmp/ca.crt
```

### Create kubeconfig

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

### Short-lived tokens

The TokenRequest API allows creating tokens with limited lifetime:

```shell
d8 k create token gitlab-runner-deploy -n ci-deploy --duration=1h
```

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
The password is sent to DKP and validated through basic-auth-proxy/Dex.
{% endalert %}

{% alert level="warning" %}
Only one DexProvider in the cluster can have `enableBasicAuth: true`.
{% endalert %}

### Prerequisites

- [publishAPI](/modules/user-authn/configuration.html#parameters-publishapi) enabled.
- [DexProvider](/modules/user-authn/cr.html#dexprovider) configured for IdP.

### Enable

Add `enableBasicAuth: true` to DexProvider. For DexProvider configuration examples, see [user-authn module documentation](/modules/user-authn/usage.html).

### Get API endpoint

```shell
API_HOST=$(d8 k -n d8-user-authn get ingress kubernetes-api -o jsonpath='{.spec.rules[0].host}')
echo "https://${API_HOST}"
```

### Verification

```shell
curl -q -u "$K8S_USER:$K8S_PASSWORD" "https://${API_HOST}/version"
```

`401` — invalid credentials or Basic Auth not enabled. `403` — authentication succeeded but RBAC denies access.

### Configure kubeconfig

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

{% raw %}

```yaml
deploy:
  script:
    - d8 k --server="$K8S_SERVER" --username="$K8S_USER" --password="$K8S_PASSWORD" get ns
```

{% endraw %}

Variables `K8S_SERVER`, `K8S_USER`, `K8S_PASSWORD` are set in the project CI/CD settings.

---

## Token Exchange

The pipeline obtains a token from the IdP, exchanges it at Dex for a token with `aud=kubernetes`, and uses it to access the API.

{% alert level="info" %}
Recommended for GitLab CI and GitHub Actions.
{% endalert %}

DKP/Dex does not receive the user password. How `IDP_TOKEN` is obtained depends on the IdP: OIDC job token (GitLab/GitHub) or IdP token endpoint (client_credentials).

### Prerequisites

- [publishAPI](/modules/user-authn/configuration.html#parameters-publishapi) enabled.
- [DexProvider](/modules/user-authn/cr.html#dexprovider) configured as **OIDC type**.

{% alert level="info" %}
Token exchange is guaranteed to work with OIDC connectors. For `type: GitLab` or `type: GitHub`, verify support in your DKP version.
{% endalert %}

### Create DexClient

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

Get client secret:

```shell
d8 k -n d8-user-authn get secret dex-client-ci-token-exchange -o jsonpath='{.data.clientSecret}' | base64 -d
```

**client_id** format: `dex-client-ci-token-exchange@d8-user-authn`.

### Get Dex and API URLs

```shell
DEX_HOST=$(d8 k -n d8-user-authn get ingress dex -o jsonpath='{.spec.rules[0].host}')
API_HOST=$(d8 k -n d8-user-authn get ingress kubernetes-api -o jsonpath='{.spec.rules[0].host}')
```

### Grant RBAC

DKP configures kube-apiserver to validate Dex tokens. The `email` and `groups` claims from the token are used for RBAC.

The set of claims required by kube-apiserver for authentication depends on configuration. If kube-apiserver requires the `name` claim, add the `profile` scope.

```shell
cat <<EOF | d8 k apply -f -
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: ci-deployer
spec:
  subjects:
  - kind: User
    name: deployer@example.com
  accessLevel: Admin
EOF
```

### Token exchange request

```shell
RESPONSE=$(curl -q -s -X POST "https://${DEX_HOST}/token" \
  -u "${DEX_CLIENT_ID}:${DEX_CLIENT_SECRET}" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=urn:ietf:params:oauth:grant-type:token-exchange" \
  -d "subject_token=${IDP_TOKEN}" \
  -d "subject_token_type=urn:ietf:params:oauth:token-type:id_token" \
  -d "connector_id=${CONNECTOR_ID}" \
  -d "scope=openid profile email groups audience:server:client_id:kubernetes" \
  -d "requested_token_type=urn:ietf:params:oauth:token-type:id_token")

DEX_TOKEN=$(echo "$RESPONSE" | jq -r '.access_token')
```

Parameters:
- `subject_token` — token from IdP
- `subject_token_type` — `id_token` (GitLab/GitHub) or `access_token` (Keycloak)
- `connector_id` — `metadata.name` of the DexProvider resource
- `scope` — must include `audience:server:client_id:kubernetes` and `profile`

{% alert level="info" %}
If the API returns 401 with an audience error, use `audience:server:client_id:<expected audience>`. Usually this is `kubernetes`.
{% endalert %}

### GitLab CI

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
        -d "connector_id=gitlab" \
        -d "scope=openid profile email groups audience:server:client_id:kubernetes" \
        -d "requested_token_type=urn:ietf:params:oauth:token-type:id_token")
      DEX_TOKEN=$(echo "$RESPONSE" | jq -r '.access_token')
      d8 k --server="${K8S_SERVER}" --token="${DEX_TOKEN}" get ns
```

{% endraw %}

### Keycloak

Get token from Keycloak:

```shell
KEYCLOAK_TOKEN=$(curl -q -s -X POST "https://<KEYCLOAK_HOST>/realms/<REALM>/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=<KEYCLOAK_CLIENT_ID>" \
  -d "client_secret=<KEYCLOAK_CLIENT_SECRET>" | jq -r '.access_token')
```

Exchange at Dex (for Keycloak, use `subject_token_type=access_token`):

{% alert level="info" %}
For access_token exchange, DexProvider requires `getUserInfo: true`.
{% endalert %}

```shell
RESPONSE=$(curl -q -s -X POST "https://${DEX_HOST}/token" \
  -u "${DEX_CLIENT_ID}:${DEX_CLIENT_SECRET}" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=urn:ietf:params:oauth:grant-type:token-exchange" \
  -d "subject_token=${KEYCLOAK_TOKEN}" \
  -d "subject_token_type=urn:ietf:params:oauth:token-type:access_token" \
  -d "connector_id=keycloak" \
  -d "scope=openid profile email groups audience:server:client_id:kubernetes" \
  -d "requested_token_type=urn:ietf:params:oauth:token-type:id_token")

DEX_TOKEN=$(echo "$RESPONSE" | jq -r '.access_token')
d8 k --server="https://${API_HOST}" --token="${DEX_TOKEN}" get ns
```

### Diagnostics

**Dex 400** — invalid `subject_token`, `subject_token_type`, or `connector_id`.

**Dex 401** — invalid client credentials or invalid subject token.

**API 401** — token validation failed. Check:
- Annotation `dexclient.deckhouse.io/allow-access-to-kubernetes` on DexClient.
- Scope contains `audience:server:client_id:kubernetes` and `profile`.
- Time synchronization between CI runner and cluster.

**API 403** — authentication succeeded but RBAC does not allow access for the user or group from the token.

Decode token to check claims:

```shell
echo "${DEX_TOKEN}" | cut -d. -f2 | base64 -d 2>/dev/null | jq .
```
