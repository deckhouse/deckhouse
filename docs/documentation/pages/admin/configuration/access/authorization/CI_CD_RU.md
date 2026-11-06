---
title: "Доступ для CI/CD"
permalink: ru/admin/configuration/access/authorization/ci_cd.html
lang: ru
description: "Настройка доступа CI/CD к API Kubernetes в Deckhouse: ServiceAccount, Basic Auth и Token Exchange."
---

Для аутентификации CI/CD-пайплайнов в API Kubernetes доступны следующие методы:
- [ServiceAccount](#serviceaccount) — токен Kubernetes ServiceAccount.
- [Basic Auth](#basic-auth) — логин и пароль через IdP.
- [Token Exchange](#обмен-токена-token-exchange) — обмен токена IdP на токен Dex.

---

## ServiceAccount

Токен ServiceAccount используется напрямую для аутентификации в API. Внешний IdP не требуется. Для ServiceAccount можно создавать [долгоживущие токены](#создание-serviceaccount-и-долгоживущего-токена) (без TTL, с возможностью отзыва или ротации) и [короткоживущие токены](#короткоживущие-токены-tokenrequest).

При совместном использовании одного ServiceAccount несколькими пайплайнами в audit-логах не будет информации о конкретном пайплайне.

### Предварительные требования

Для настройки аутентификации через токен для ServiceAccount должны быть соблюдены следующие требования:

- Доступ к кластеру с правами на создание ServiceAccount и секретов.
- Для внешнего доступа: опубликованный через Ingress API-сервер Kubernetes (для публикации используется параметр [apiserver.publishAPI](/modules/control-plane-manager/configuration.html#parameters-apiserver-publishapi) модуля `control-plane-manager`) или прямой доступ к API через VPN.

### Создание ServiceAccount и долгоживущего токена

Для создания ServiceAccount и токена используйте команду:

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
Секрет типа `kubernetes.io/service-account-token` — устаревший подход. Рекомендуется использовать TokenRequest API (`d8 k create token ...`).
{% endalert %}

### Выдача прав

Подробнее о выдаче прав — в разделе [«Выдача прав пользователям и сервисным аккаунтам»](granting.html).

Назначьте необходимые для ServiceAccount права.

Для упрощённой ролевой модели используйте [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule):

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

Для гранулярной ролевой модели выдайте роль `d8:project:admin` во всех проектах с помощью [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding). Чтобы выдать доступ только к одному неймспейсу, вместо этого создайте в нём RoleBinding на роль `d8:namespace:admin`:

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

### Получение URL API и сертификата CA

При использовании publishAPI:

{% tabs api_publish_type %}
{% tab "При публикации API-сервера через Ingress" %}

При публикации API-сервера через Ingress используйте команды:

```shell
API_HOST=$(d8 k -n kube-system get ingress kubernetes-api -o jsonpath='{.spec.rules[0].host}')
echo "API endpoint: https://${API_HOST}"
```

{% endtab %}
{% tab "При публикации API-сервера через Gateway API (`alb`)" %}

При публикации API-сервера через Gateway API (модуль [`alb`](/modules/alb/)) используйте команды:

```shell
API_HOST=$(d8 k -n kube-system get httproute kubernetes-api -o jsonpath='{.spec.hostnames[0]}')
echo "API endpoint: https://${API_HOST}"
```

{% endtab %}
{% endtabs %}

{% alert level="info" %}
Если сертификат API подписан публичным CA (Let's Encrypt), параметр `--certificate-authority` не требуется.
{% endalert %}

Для приватного CA (self-signed, режим по умолчанию — общий для обоих способов публикации):

```shell
d8 k -n kube-system get secret kubernetes-api-ca-key-pair -o jsonpath='{.data.tls\.crt}' | base64 -d > /tmp/ca.crt
```

### Создание kubeconfig

Пример генерации kubeconfig для созданного ранее ServiceAccount:

```shell
export CLUSTER_NAME=my-cluster
export USER_NAME=gitlab-runner-deploy
export CONTEXT_NAME=${CLUSTER_NAME}-${USER_NAME}
export FILE_NAME=kube.config

# Для публичного CA уберите --certificate-authority и --embed-certs.
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

### Короткоживущие токены (TokenRequest)

TokenRequest API позволяет создавать токены с ограниченным сроком действия. При создании такого токена укажите в команде имя ServiceAccount, которому нужно предоставить доступ (в примере — `gitlab-runner-deploy`):

```shell
d8 k create token gitlab-runner-deploy -n ci-deploy --duration=1h
```

{% alert level="warning" %}
Команда `d8 k create token` сама требует доступа к Kubernetes API. Если CI-задача (job) не имеет начального доступа к кластеру, выполнить TokenRequest внутри задачи невозможно.
{% endalert %}

#### Типовые сценарии к использованию в CI

**Bootstrap-токен с минимальными правами.** CI хранит долгоживущий kubeconfig/токен, который имеет право только на создание TokenRequest для конкретного ServiceAccount. В начале задачи bootstrap-токен выпускает короткоживущий токен, который используется для деплоя.

**Внешний token broker.** Оператор, секрет-менеджер (Vault, External Secrets) или внешний сервис периодически выпускает короткоживущие токены и помещает их в CI-переменные или секреты. CI-задача только использует готовый токен.

{% alert level="info" %}
Bootstrap-токен должен иметь минимальные права: только `create` на ресурс `serviceaccounts/token` для конкретного ServiceAccount/namespace. Не используйте cluster-admin. Ограничивайте TTL выпускаемых токенов и оперативно отзывайте bootstrap-доступ при его компрометации.
{% endalert %}

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
Пароль передаётся в DP и проверяется через basic-auth-proxy/Dex.
{% endalert %}

{% alert level="warning" %}
Только один DexProvider в кластере может иметь `enableBasicAuth: true`.
{% endalert %}

### Предварительные требования

Для настройки Basic Auth должны быть соблюдены следующие требования:

- [apiserver.publishAPI](/modules/control-plane-manager/configuration.html#parameters-apiserver-publishapi) модуля `control-plane-manager` включён.
- [DexProvider](/modules/user-authn/cr.html#dexprovider) настроен для IdP.

### Включение

В DexProvider добавьте `enableBasicAuth: true`. Пример настройки DexProvider приведён [в документации модуля user-authn](/modules/user-authn/usage.html).

### Получение эндпоинта API

Для получения эндпоинта API используйте команду:

```shell
API_HOST=$(d8 k -n d8-user-authn get ingress kubernetes-api -o jsonpath='{.spec.rules[0].host}')
echo "https://${API_HOST}"
```

### Проверка

Для проверки доступа используйте команду:

```shell
curl -q -u "$K8S_USER:$K8S_PASSWORD" "https://${API_HOST}/version"
```

Возможные ошибки:

- `401` — неверные учётные данные или Basic Auth не включён.
- `403` — аутентификация прошла, но RBAC запрещает доступ.

### Настройка kubeconfig

Пример настройки kubeconfig для Basic Auth:

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

Пример использования Basic Auth в GitLab CI:

{% raw %}

```yaml
deploy:
  script:
    - d8 k --server="$K8S_SERVER" --username="$K8S_USER" --password="$K8S_PASSWORD" get ns
```

{% endraw %}

Переменные `K8S_SERVER`, `K8S_USER`, `K8S_PASSWORD` задаются в настройках CI/CD [проекта](../../../../glossary.html#project).

---

## Обмен токена (token exchange)

Пайплайн получает токен от CI-системы, обменивает его в Dex на токен с `aud=kubernetes` и обращается с ним к API.

{% alert level="info" %}
Рекомендуется для GitLab CI и GitHub Actions.
{% endalert %}

DP/Dex не получает пароль. Dex доверяет CI-системе как [TokenExchangeProvider](/modules/user-authn/cr.html#tokenexchangeprovider). Dex проверяет подпись, издателя, срок действия и аудиторию токена, а также обязательные claims, и называет пайплайн в кластере по claim токена.

### Предварительные требования

Для настройки Token Exchange должны быть соблюдены следующие требования:

- [apiserver.publishAPI](/modules/control-plane-manager/configuration.html#parameters-apiserver-publishapi) модуля `control-plane-manager` включён.
- CI-система выпускает подписанные JWT ID-токены (`id_tokens` в GitLab CI, OIDC в GitHub Actions) и публикует discovery-документ, либо у вас есть её открытые ключи.

### Создание DexClient

Для создания DexClient выполните команду:

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

Аннотация `dexclient.deckhouse.io/allow-access-to-kubernetes` разрешает клиенту запрашивать токены с `aud=kubernetes`.

{% alert level="warning" %}
Это даёт доступ к API Kubernetes на уровне всего кластера. Поэтому для добавления аннотации или изменения её значения на `"true"` необходимо право `create` на подресурс `dexclients/allow-access-to-kubernetes` группы API `deckhouse.io` в неймспейсе DexClient. В упрощённой ролевой модели это право выдаётся на уровне `ClusterAdmin` и выше. В гранулярной модели его выдаёт роль `d8:subsystem:iam:manager` и включающие её роли, например `d8:system:manager`. Роли уровня неймспейса, разрешающей создавать DexClient, недостаточно: политика admission отклонит запрос с сообщением, в котором названа аннотация.

Добавление аннотации ограничено независимо от указанного значения, включая `"false"`. Это нужно для совместимости с предыдущими версиями DP, в которых доступ выдавался при наличии аннотации, независимо от её значения.

Ограничение не действует для объекта, у которого аннотация уже есть. Удаление и повторное создание DexClient считается новым добавлением аннотации, поэтому GitOps-контроллеру, который пересоздаёт объект, а не изменяет его, нужно то же право `create` на подресурс `dexclients/allow-access-to-kubernetes`.
{% endalert %}

Получите секрет клиента:

```shell
d8 k -n d8-user-authn get secret dex-client-ci-token-exchange -o jsonpath='{.data.clientSecret}' | base64 -d
```

Формат **client_id**: `dex-client-ci-token-exchange@d8-user-authn`.

### Создание TokenExchangeProvider

TokenExchangeProvider задаёт, какие токены принимает Dex и какому DexClient разрешено их обменивать. Пример для GitLab CI:

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

Параметры:

- `issuer` — URL-адрес инсталляции GitLab, как в claim `iss` её ID-токенов.
- `audiences` — значение `aud`, которое задание указывает в `id_tokens`. Токен, выпущенный для другого сервиса, отклоняется.
- `claimValidation` — claims, которые обязан нести токен. В примере принимаются только защищённые ветки [проектов](../../../../glossary.html#project) группы `platform`.
- `identity.username` — имя пользователя в кластере, например `gitlab-ci:project_path:platform/app:ref_type:branch:ref:main` для ветки `main` проекта `platform/app`.

Получите идентификатор коннектора для запроса обмена:

```shell
CONNECTOR_ID=$(d8 k get tokenexchangeprovider gitlab-ci -o jsonpath='{.status.connectorID}')
```

Сохраните его в переменной CI. Идентификатор меняется, если объект удалить и создать заново, а также если провайдер получает коннектор обратно после `PrefixConflict`. В этих случаях обновите переменную, иначе Dex ответит 400 `Requested connector does not exist`.

### Получение URL Dex и API

Для получения URL Dex и API выполните команды:

```shell
DEX_HOST=$(d8 k -n d8-user-authn get ingress dex -o jsonpath='{.spec.rules[0].host}')
API_HOST=$(d8 k -n d8-user-authn get ingress kubernetes-api -o jsonpath='{.spec.rules[0].host}')
```

### Выдача RBAC

DP настраивает kube-apiserver на проверку токенов Dex. Имя пользователя — то, что задаёт в TokenExchangeProvider `identity.email`, если он указан, а иначе `identity.username`; группы берутся из `identity.groups`.

Подробнее о выдаче прав — в разделе [«Выдача прав пользователям и сервисным аккаунтам»](granting.html).

Для упрощённой [ролевой модели](../../../../glossary.html#rbac) используйте [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule):

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

Для гранулярной ролевой модели выдайте роль `d8:project:admin` во всех проектах с помощью [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding). Чтобы выдать доступ только к одному неймспейсу, создайте в нём RoleBinding на роль `d8:namespace:admin`:

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

### Запрос обмена токена

Пример запроса обмена токена:

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

Параметры:

- `subject_token` — ID-токен CI-системы. Access-токены не принимаются.
- `connector_id` — `status.connectorID` объекта TokenExchangeProvider.
- `scope` — должен содержать следующие значения:
  - `openid` — нужен для ID-токена;
  - `email` — kube-apiserver берёт имя пользователя из claim `email`, без него API ответит 401;
  - `audience:server:client_id:kubernetes` — токен выпускается для kube-apiserver;
  - `groups` — нужен, если RBAC выдаёт права группам;
  - `federated:id` — показывает TokenExchangeProvider токена в `d8 k auth whoami` (атрибут extra `user-authn.deckhouse.io/dex-provider`).

Токен Dex действует в течение `idTokenTTL` модуля `user-authn` (по умолчанию 10 минут). Для более долгого задания запросите новый токен.

### GitLab CI

Пример запроса токена в GitLab CI:

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

Для GitHub Actions задайте в TokenExchangeProvider:

- `issuer` — `https://token.actions.githubusercontent.com`.
- `audiences` — значение, которое рабочий процесс передаёт как `audience`.
- `claimValidation` — `repository_owner_id` и при необходимости `ref`. GitHub рекомендует проверять неизменяемые идентификаторы. Имя переименованной или удалённой организации может занять кто-то другой.
- `identity.username` — `claim: sub` с префиксом, например `github:`. Имя пользователя будет вида `github:repo:my-org/app:ref:refs/heads/main`.

Чтобы запросить токен, добавьте в рабочий процесс право `id-token: write`:

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

Обменяйте `CI_TOKEN`, как описано в разделе [«Запрос обмена токена»](#запрос-обмена-токена).

### Переход с DexProvider

Раньше токен обменивался через DexProvider типа OIDC, и запрос обмена указывал его в `connector_id`. Этот способ устарел и по умолчанию выключен. DexProvider обменивает токены, только если задано `spec.tokenExchange.enabled: true`. Модуль выставил это значение при обновлении в кластерах, где был DexClient с аннотацией `dexclient.deckhouse.io/allow-access-to-kubernetes`. Параметр можно только выключить. Он будет удалён после следующего LTS-релиза вместе с этим способом. Пока в кластере идёт обмен через DexProvider, срабатывает алерт `D8UserAuthnDexProviderTokenExchangeDeprecated`.

Для перехода выполните следующие шаги:

1. Создайте TokenExchangeProvider с `issuer` объекта DexProvider и аудиторией, которую несут токены вашей CI-системы.
1. Задайте в `identity.username` claim, из которого DexProvider брал имя пользователя, например `claim: user_email` для GitLab CI, с префиксом, например `gitlab-ci:`; группам тоже задайте префикс. Префикс обязателен. Он отделяет субъектов провайдера от людей и от субъектов других провайдеров.
1. Сохраните проверки DexProvider:
   - DexProvider принимал только токен с `email_verified: true`, если не был задан `insecureSkipEmailVerified`. Если имя пользователя берётся из email, добавьте `claimValidation: [{claim: email_verified, equals: "true"}]`.
   - Перенесите `allowedEmails`, `allowedEmailDomains` и `allowedGroups` в `allowedIdentities`. Dex применяет их к именам с добавленными префиксами, например `gitlab-ci:deployer@example.com`. Добавьте префиксы из `identity` к `allowedEmails` и `allowedGroups`. Значения `allowedEmailDomains` перенесите без изменений, потому что домен — часть после последнего `@`.
   - Для `allowedGroups` нужен `identity.groups`. Без групп любой обмен отклоняется.
1. До переключения добавьте субъектов с префиксом в объекты ClusterAuthorizationRule, AuthorizationRule и RoleBinding, которые выдают права старым именам, например `gitlab-ci:deployer@example.com` рядом с `deployer@example.com`.
1. В запросе обмена замените `connector_id` на `status.connectorID` нового объекта.
1. После переключения удалите старых субъектов из этих объектов.
1. Выключите обмен токенов через DexProvider, установив `spec.tokenExchange.enabled: false`. Включить его снова нельзя.

TokenExchangeProvider принимает только ID-токены в формате JWT. Пайплайн, который обменивал access-токен, например Keycloak, должен запрашивать у издателя ID-токен.

### Устранение неполадок

{% alert level="info" %}
Используйте `d8 k auth whoami` для проверки под какой идентичностью выполняется запрос.
{% endalert %}

**Флаг `--token` может не работать** — kubeconfig с `auth-provider`, `exec` или `tokenFile` переопределяет токен из флага. Для отладки используйте `-v=8`:

```shell
d8 k --server="https://${API_HOST}" --token="${DEX_TOKEN}" auth whoami -v=8
```

В выводе будет видно какой Authorization заголовок отправляется. Для надёжной работы с токеном используйте минимальный kubeconfig только с `server` и `token`, без `auth-provider`/`exec`.

**TokenExchangeProvider не готов** — проверьте условие `Ready` объекта:

```shell
d8 k get tokenexchangeprovider gitlab-ci -o jsonpath='{.status.conditions}'
```

**Dex 400** — одна из причин:

- неизвестный `subject_token_type` или неверный `connector_id` (`Requested connector does not exist`);
- DexClient не указан в `spec.clients` (`Connector not allowed for this client.`);
- `invalid_scope` — у DexClient нет аннотации `dexclient.deckhouse.io/allow-access-to-kubernetes`, поэтому он не может запрашивать `audience:server:client_id:kubernetes`. Если аннотации нет, проверьте, не отклонил ли admission-контроллер запрос, который должен был её добавить, и не пересоздал ли GitOps-контроллер объект без неё. Для добавления аннотации необходимо право `create` на подресурс `dexclients/allow-access-to-kubernetes` группы API `deckhouse.io` в неймспейсе DexClient;
- `unauthorized_client` — клиент является kubeconfig-клиентом или `kubernetes`, им обмен токенов запрещён.

**Dex 401** — неверные учётные данные клиента, либо токен отклонён: access-токен вместо ID-токена, неверная подпись или издатель, `kid` не совпал ни с одним заданным ключом, истёк срок или срок действия дольше `subjectTokenMaxLifetime`, другая аудитория, не совпал обязательный claim или имена с добавленными префиксами не входят в `allowedIdentities` (`User is not allowed.`). Причина записывается в журнал Dex.

**API 401** — токен не прошёл проверку. Проверьте:

- Параметр `scope` содержит `email` и `audience:server:client_id:kubernetes`.
- Синхронизация времени между CI-раннером и кластером.

**API 403** — аутентификация прошла, но RBAC не разрешает доступ для пользователя или группы из токена, либо TokenExchangeProvider удалён или потерял коннектор (`PrefixConflict`). В этом случае все выданные через него токены отклоняются, если в модуле `user-authz` включён параметр `enableMultiTenancy` и в кластере работает Kubernetes 1.34 или новее.

Декодирование токена для проверки claims:

```shell
echo "${DEX_TOKEN}" | cut -d. -f2 | base64 -d 2>/dev/null | jq .
```
