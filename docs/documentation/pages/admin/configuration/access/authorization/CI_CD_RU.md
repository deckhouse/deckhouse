---
title: "Доступ для CI/CD"
permalink: ru/admin/configuration/access/authorization/ci_cd.html
lang: ru
description: "Настройка доступа CI/CD к API Kubernetes в Deckhouse: ServiceAccount, Basic Auth и Token Exchange."
---

Для аутентификации CI/CD-пайплайнов в API Kubernetes доступны три метода:
- [ServiceAccount](#serviceaccount) — токен Kubernetes ServiceAccount.
- [Basic Auth](#basic-auth) — логин и пароль через IdP.
- [Token Exchange](#token-exchange) — обмен токена IdP на токен Dex.

---

## ServiceAccount

Токен ServiceAccount используется напрямую для аутентификации в API. Внешний IdP не требуется.

При совместном использовании одного ServiceAccount несколькими пайплайнами в audit-логах не будет информации о конкретном пайплайне.

### Предварительные требования

- Доступ к кластеру с правами на создание ServiceAccount и Secret.
- Для внешнего доступа: [publishAPI](/modules/user-authn/configuration.html#parameters-publishapi) или прямой доступ к API через VPN.

### Создание ServiceAccount и токена

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
Secret типа `kubernetes.io/service-account-token` — legacy-подход. Рекомендуется использовать TokenRequest API (`d8 k create token ...`).
{% endalert %}

### Выдача прав

Подробнее о выдаче прав см. в разделе [Выдача прав пользователям и сервисным аккаунтам](granting.html).

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

Доступные уровни: `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`, `ClusterAdmin`, `SuperAdmin`.

Экспериментальная ролевая модель — [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/):

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

### Получение URL API и сертификата CA

При использовании publishAPI:

```shell
API_HOST=$(d8 k -n d8-user-authn get ingress kubernetes-api -o jsonpath='{.spec.rules[0].host}')
echo "API endpoint: https://${API_HOST}"
```

{% alert level="info" %}
Если сертификат API подписан публичным CA (Let's Encrypt), параметр `--certificate-authority` не требуется.
{% endalert %}

Для приватного CA:

```shell
d8 k -n d8-user-authn get secret kubernetes-api-ca-key-pair -o jsonpath='{.data.ca\.crt}' | base64 -d > /tmp/ca.crt
```

### Создание kubeconfig

```shell
export CLUSTER_NAME=my-cluster
export USER_NAME=gitlab-runner-deploy
export CONTEXT_NAME=${CLUSTER_NAME}-${USER_NAME}
export FILE_NAME=kube.config

# Для публичного CA уберите --certificate-authority и --embed-certs
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

### Короткоживущие токены

TokenRequest API позволяет создавать токены с ограниченным сроком действия:

```shell
d8 k create token gitlab-runner-deploy -n ci-deploy --duration=1h
```

Пример использования без файла kubeconfig:

```shell
export KUBE_SERVER="https://${API_HOST}"
export KUBE_TOKEN=$(d8 k create token gitlab-runner-deploy -n ci-deploy --duration=1h)
d8 k --server=$KUBE_SERVER --token=$KUBE_TOKEN get ns
```

---

## Basic Auth

Аутентификация по логину и паролю через IdP (LDAP, OIDC).

{% alert level="warning" %}
Пароль передаётся в DKP и проверяется через basic-auth-proxy/Dex.
{% endalert %}

{% alert level="warning" %}
Только один DexProvider в кластере может иметь `enableBasicAuth: true`.
{% endalert %}

### Предварительные требования

- [publishAPI](/modules/user-authn/configuration.html#parameters-publishapi) включён.
- [DexProvider](/modules/user-authn/cr.html#dexprovider) настроен для IdP.

### Включение

В DexProvider добавьте `enableBasicAuth: true`. Пример настройки DexProvider см. в [документации модуля user-authn](/modules/user-authn/usage.html).

### Получение endpoint API

```shell
API_HOST=$(d8 k -n d8-user-authn get ingress kubernetes-api -o jsonpath='{.spec.rules[0].host}')
echo "https://${API_HOST}"
```

### Проверка

```shell
curl -q -u "$K8S_USER:$K8S_PASSWORD" "https://${API_HOST}/version"
```

`401` — неверные учётные данные или Basic Auth не включён. `403` — аутентификация прошла, но RBAC запрещает доступ.

### Настройка kubeconfig

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

### Использование в GitLab CI

{% raw %}

```yaml
deploy:
  script:
    - d8 k --server="$K8S_SERVER" --username="$K8S_USER" --password="$K8S_PASSWORD" get ns
```

{% endraw %}

Переменные `K8S_SERVER`, `K8S_USER`, `K8S_PASSWORD` задаются в настройках CI/CD проекта.

---

## Token Exchange

Пайплайн получает токен от IdP, обменивает его в Dex на токен с `aud=kubernetes` и обращается с ним к API.

{% alert level="info" %}
Рекомендуется для GitLab CI и GitHub Actions.
{% endalert %}

DKP/Dex не получает пароль пользователя. Способ получения `IDP_TOKEN` зависит от IdP: OIDC job token (GitLab/GitHub) или token endpoint IdP (client_credentials).

### Предварительные требования

- [publishAPI](/modules/user-authn/configuration.html#parameters-publishapi) включён.
- [DexProvider](/modules/user-authn/cr.html#dexprovider) настроен как **тип OIDC**.

{% alert level="info" %}
Token exchange гарантированно работает с OIDC-коннекторами. Для `type: GitLab` или `type: GitHub` проверьте поддержку в вашей версии DKP.
{% endalert %}

### Создание DexClient

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

Аннотация `dexclient.deckhouse.io/allow-access-to-kubernetes` позволяет клиенту запрашивать токены с `aud=kubernetes`.

Получение client secret:

```shell
d8 k -n d8-user-authn get secret dex-client-ci-token-exchange -o jsonpath='{.data.clientSecret}' | base64 -d
```

Формат **client_id**: `dex-client-ci-token-exchange@d8-user-authn`.

### Получение URL Dex и API

```shell
DEX_HOST=$(d8 k -n d8-user-authn get ingress dex -o jsonpath='{.spec.rules[0].host}')
API_HOST=$(d8 k -n d8-user-authn get ingress kubernetes-api -o jsonpath='{.spec.rules[0].host}')
```

### Выдача RBAC

DKP настраивает kube-apiserver на проверку токенов Dex. Claims `email` и `groups` из токена используются для RBAC.

Набор claims, которые требуются kube-apiserver для аутентификации, зависит от конфигурации. Если kube-apiserver требует `name`, добавьте scope `profile`.

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

### Запрос обмена токена

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

Параметры:
- `subject_token` — токен от IdP
- `subject_token_type` — `id_token` (GitLab/GitHub) или `access_token` (Keycloak)
- `connector_id` — `metadata.name` ресурса DexProvider
- `scope` — обязательно `audience:server:client_id:kubernetes` и `profile`

{% alert level="info" %}
Если API возвращает 401 с ошибкой audience, используйте `audience:server:client_id:<ожидаемая audience>`. Обычно это `kubernetes`.
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

Получение токена из Keycloak:

```shell
KEYCLOAK_TOKEN=$(curl -q -s -X POST "https://<KEYCLOAK_HOST>/realms/<REALM>/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=<KEYCLOAK_CLIENT_ID>" \
  -d "client_secret=<KEYCLOAK_CLIENT_SECRET>" | jq -r '.access_token')
```

Обмен в Dex (для Keycloak используется `subject_token_type=access_token`):

{% alert level="info" %}
Для обмена access_token в DexProvider требуется `getUserInfo: true`.
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

### Диагностика

**Dex 400** — неверный `subject_token`, `subject_token_type` или `connector_id`.

**Dex 401** — неверные учётные данные клиента или невалидный subject token.

**API 401** — токен не прошёл проверку. Проверьте:
- Аннотация `dexclient.deckhouse.io/allow-access-to-kubernetes` на DexClient.
- Scope содержит `audience:server:client_id:kubernetes` и `profile`.
- Синхронизация времени между CI-раннером и кластером.

**API 403** — аутентификация прошла, но RBAC не разрешает доступ для пользователя или группы из токена.

Декодирование токена для проверки claims:

```shell
echo "${DEX_TOKEN}" | cut -d. -f2 | base64 -d 2>/dev/null | jq .
```
