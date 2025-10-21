---
title: "Механизм секретов Kubernetes"
permalink: ru/stronghold/documentation/user/secrets-engines/kubernetes.html
lang: ru
---

Kubernetes Secrets Engine для Stronghold генерирует токены для учетной записи сервиса Kubernetes
(не путать с [токенами Stronghold](../concepts/tokens.html)), а также, по желанию, сами объекты учетной записи сервиса `ServiceAccount`,
роли `Role` и привязку роли к учетной записи сервиса `RoleBinding`. Созданные токены имеют настраиваемый [срок жизни (TTL)](#token-ttl), а все созданные объекты автоматически удаляются по истечении срока [аренды (lease)](../concepts/lease.html) Stronghold.

На каждую аренду Stronghold создает токен под конкретную учетную запись сервиса. Токен возвращается вызывающей стороне.

Для большей информации о ресурсах Kubernetes ознакомьтесь с официальной документацией [Kubernetes service account](https://kubernetes.io/docs/concepts/security/service-accounts/)
и [Kubernetes RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/).

{% alert level="warning" %}
Мы не рекомендуем использовать токены, созданные механизмом секретов Kubernetes, для аутентификации с помощью [Kubernetes Auth Method](../auth/kubernetes.html). Это приведет к созданию множества уникальных идентификаторов в Stronghold, которыми будет сложно управлять.
{% endalert %}

## Настройка

Перед использованием механизма секретов Kubernetes, необходимо предварительно его настроить.
Эти шаги обычно выполняются администратором Stronghold или инструментом автоматического управления конфигурацией.

По умолчанию Stronghold подключается к Kubernetes, используя собственную учетную запись сервиса.
При использовании [Helm chart](https://github.com/hashicorp/vault-helm) эта учетная запись сервиса
создается автоматически по умолчанию и называется по имени релиза Helm (по умолчанию `stronghold`,
но это можно настроить через значение Helm `server.serviceAccount.name`).

Необходимо убедиться, что учетная запись сервиса, которую использует Stronghold, будет иметь
права на управление токенами учетных записей сервиса, а также при использовании функционала и на управление учетными записями сервиса,
ролями и привязками ролей. Этими правами можно управлять с помощью тех же ролей Kubernetes.
Роль привязывается к учетной записи сервиса Stronghold с помощью привязки роли или привязки кластерной роли.

Например, роль кластера только для создания токенов учетных записей сервиса:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: k8s-minimal-secrets-abilities
rules:
- apiGroups: [""]
  resources: ["serviceaccounts/token"]
  verbs: ["create"]
```

Аналогичным образом можно создать кластерную роль с бóльшими правами. В данном случае
установлены права на управление токенами, учетными записями сервиса, привязками ролей к учетным записям сервиса и ролями.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: k8s-full-secrets-abilities
rules:
- apiGroups: [""]
  resources: ["serviceaccounts", "serviceaccounts/token"]
  verbs: ["create", "update", "delete"]
- apiGroups: ["rbac.authorization.k8s.io"]
  resources: ["rolebindings", "clusterrolebindings"]
  verbs: ["create", "update", "delete"]
- apiGroups: ["rbac.authorization.k8s.io"]
  resources: ["roles", "clusterroles"]
  verbs: ["bind", "escalate", "create", "update", "delete"]
```

Создайте эту роль в Kubernetes (например, с помощью `d8 k apply -f`).

Более того, если вы хотите использовать ограничение выбор меток (label) для возможности выборки пространств имен,
в которых может действовать роль, вам нужно будет предоставить разрешение Stronghold на чтение пространств имен.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: k8s-full-secrets-abilities-with-labels
rules:
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get"]
- apiGroups: [""]
  resources: ["serviceaccounts", "serviceaccounts/token"]
  verbs: ["create", "update", "delete"]
- apiGroups: ["rbac.authorization.k8s.io"]
  resources: ["rolebindings", "clusterrolebindings"]
  verbs: ["create", "update", "delete"]
- apiGroups: ["rbac.authorization.k8s.io"]
  resources: ["roles", "clusterroles"]
  verbs: ["bind", "escalate", "create", "update", "delete"]
```

{% alert level="warning" %}
Получение правильных разрешений для Stronghold, скорее всего, потребует проб и ошибок, поскольку Kubernetes имеет строгую защиту от повышения привилегий.
Подробнее об этом можно прочитать в документации [Kubernetes RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#privilege-escalation-prevention-and-bootstrapping)
{% endalert %}

{% alert level="warning" %}
Защитите учетную запись сервиса Stronghold, особенно если вы используете для нее широкие права, поскольку она по сути является учетной записью администратора кластера.
{% endalert %}

Создайте привязку роли, чтобы связать ее с учетной записью сервиса Stronghold и предоставить Stronghold разрешение на управление токенами.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
 name: stronghold-token-creator-binding
roleRef:
 apiGroup: rbac.authorization.k8s.io
 kind: ClusterRole
 name: k8s-minimal-secrets-abilities
subjects:
- kind: ServiceAccount
 name: stronghold
 namespace: stronghold
```

Для получения дополнительной информации о ролях Kubernetes, учетных записях сервиса, привязках и токенах посетите раздел документации
[Kubernetes RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/).

Если Stronghold не будет автоматически управлять ролями или учетными записями сервиса
(см. раздел [Автоматическое управление ролями и учетными записями сервиса](#roles-and-sa)), то вам
необходимо настроить учетную запись сервиса, для которой Stronghold будет выпускать токены.

{% alert level="warning" %} Настоятельно рекомендуется, чтобы учетная запись сервиса, для которой Stronghold выпускает токены, **НЕ** совпадала с учетной записью сервиса, которую использует сам Stronghold.
{% endalert %}

Примеры, которые мы будем использовать, будут находиться в пространстве
имен `test`, которое вы можете создать, если оно еще не существует.

```shell-session
$ d8 k create namespace test
namespace/test created
```

Здесь представлена простая настройка учетной записи сервиса, роли и
привязки роли в пространстве имен Kubernetes `test` с базовыми разрешениями,
которые мы будем использовать:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
 name: test-service-account-with-generated-token
 namespace: test
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
 name: test-role-list-pods
 namespace: test
rules:
- apiGroups: [""]
 resources: ["pods"]
 verbs: ["list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
 name: test-role-abilities
 namespace: test
roleRef:
 apiGroup: rbac.authorization.k8s.io
 kind: Role
 name: test-role-list-pods
subjects:
- kind: ServiceAccount
 name: test-service-account-with-generated-token
 namespace: test
```

Вы можете создать эти объекты с помощью команды `d8 k apply -f`.

Включите механизм секретов Kubernetes:

```shell-session
$ stronghold secrets enable kubernetes
Success! Enabled the kubernetes Secrets Engine at: kubernetes/
```

По умолчанию движок секретов будет монтироваться по тому же имени, что и его название,
т.е. `kubernetes/`. Это можно изменить, передав аргумент `-path` при включении.

Настройте точку монтирования. Допускается пустая конфигурация.

```shell-session
stronghold write -f kubernetes/config
```

1. Теперь можно настроить роль Stronghold в механизме секретов Kubernetes
(**не** то же самое, что роль Kubernetes), которая сможет генерировать токены Kubernetes для установленной нами учетной записи сервиса:

```shell-session
$ stronghold write kubernetes/roles/my-role \
   allowed_kubernetes_namespaces="*" \
   service_account_name="test-service-account-with-generated-token" \
   token_default_ttl="10m"
```

## Создание учетных данных

После того как пользователь прошел аутентификацию в Stronghold и получил достаточные права,
запись в конечную точку `creds` для роли Stronghold сгенерирует и вернет новый токен учетной записи сервиса.

```shell-session
$ stronghold write kubernetes/creds/my-role \
    kubernetes_namespace=test

Key                        Value
–--                        -----
lease_id                   kubernetes/creds/my-role/31d771a6-...
lease_duration             10m0s
lease_renwable             false
service_account_name       test-service-account-with-generated-token
service_account_namespace  test
service_account_token      eyJHbGci0iJSUzI1NiIsImtpZCI6ImlrUEE...
```

Вы можете использовать указанный выше токен учетной записи сервиса (`eyJHbG...`) для любого
авторизованного запроса к Kubernetes API. Авторизацией управляют привязки ролей к учетной записи сервиса.

```shell-session
$ curl -sk $(d8 k config view --minify -o 'jsonpath={.clusters[].cluster.server}')/api/v1/namespaces/test/pods \
    --header "Authorization: Bearer eyJHbGci0iJSUzI1Ni..."
{
  "kind": "PodList",
  "apiVersion": "v1",
  "metadata": {
    "resourceVersion": "1624"
  },
  "items": []
}
```

После истечения срока [аренды](../concepts/lease.html), можно удостовериться, что токен был отозван и больше не может быть использован для запросов к Kubernetes API.

```shell-session
$ curl -sk $(d8 k config view --minify -o 'jsonpath={.clusters[].cluster.server}')/api/v1/namespaces/test/pods \
    --header "Authorization: Bearer eyJHbGci0iJSUzI1Ni..."
{
  "kind": "Status",
  "apiVersion": "v1",
  "metadata": {},
  "status": "Failure",
  "message": "Unauthorized",
  "reason": "Unauthorized",
  "code": 401
}
```

## Время жизни токена (TTL) {#token-ttl}

Токены учетной записи сервиса Kubernetes имеют время жизни (TTL). Когда срок
действия токена истекает, токен автоматически отзывается.

Можно установить стандартное (`token_default_ttl`) и максимальное время
жизни (`token_max_ttl`) при создании или настройке роли Stronghold.

```shell-session
$ stronghold write kubernetes/roles/my-role \
    allowed_kubernetes_namespaces="*" \
    service_account_name="new-service-account-with-generated-token" \
    token_default_ttl="10m" \
    token_max_ttl="2h"
```

Вы также можете задать время жизни (`ttl`) при генерации токена из конечной точки `creds`.
Если время жизни токена не указан, он будет использоваться по умолчанию (и не может превышать максимальный срок (`token_max_ttl`) роли, если он есть).

```shell-session
$ stronghold write kubernetes/creds/my-role \
    kubernetes_namespace=test \
    ttl=20m

Key                        Value
–--                        -----
lease_id                   kubernetes/creds/my-role/31d771a6-...
lease_duration             20m0s
lease_renwable             false
service_account_name       new-service-account-with-generated-token
service_account_namespace  test
service_account_token      eyJHbGci0iJSUzI1NiIsImtpZCI6ImlrUEE...
```

Можно проверить время жизни токена JWT-токена. Для этого декодируем токен и конвертируем
поля `iat` (issued at) и `exp` (expiration time) из формата timestamp в удобночитаемый.

```shell-session
$ echo 'eyJhbGc...' | cut -d'.' -f2 | base64 -d  | jq -r '.iat,.exp|todate'
2022-05-20T17:14:50Z
2022-05-20T17:34:50Z
```

## Аудитория (aud)

Токены в Kubernetes имеют формат JWT, а значит, используют механизм "утверждений" (claims).
Одним из таких является утверждение `aud` (аудитория) - это строка или массив строк, которые
идентифицируют получателей, для которых предназначен JWT. Для более подробной информации ознакомьтесь со спецификацией [JWT audience claim](https://datatracker.ietf.org/doc/html/rfc7519#section-4.1.3)

Вы можете задать аудитории по умолчанию (`token_default_audiences`) при создании или настройке роли Stronghold.
Если явно не задано, по умолчанию кластер Kubernetes будет использовать свои значения аудитории для токенов учетной записи сервиса.

```shell-session
$ stronghold write kubernetes/roles/my-role \
    allowed_kubernetes_namespaces="*" \
    service_account_name="new-service-account-with-generated-token" \
    token_default_audiences="custom-audience"
```

Вы также можете задать аудитории (`audiences`) при генерации токена из конечной точки `creds`.
Если аудитории токена не заданы, то они будут заданы по умолчанию из значения поля `token_default_audiences`, которое мы указывали ранее.

```shell-session
$ stronghold write kubernetes/creds/my-role \
    kubernetes_namespace=test \
    audiences="another-custom-audience"

Key                        Value
–--                        -----
lease_id                   kubernetes/creds/my-role/SriWQf0bPZ...
lease_duration             768h
lease_renwable             false
service_account_name       new-service-account-with-generated-token
service_account_namespace  test
service_account_token      eyJHbGci0iJSUzI1NiIsImtpZCI6ImlrUEE...
```

Аудиторию токена можно проверить, расшифровав JWT.

```shell-session
$ echo 'eyJhbGc...' | cut -d'.' -f2 | base64 -d
{"aud":["another-custom-audience"]...
```

## Автоматическое управление ролями и учетными записями сервиса {#roles-and-sa}

При настройке роли Stronghold вы можете передать параметры, чтобы указать, что
вы хотите автоматически генерировать учетные записи сервиса и привязку
роли, а также, по желанию, генерировать саму роль Kubernetes.

Если вы хотите настроить роль Stronghold на использование уже существующей роли
Kubernetes, но при этом автоматически создать учетную запись сервиса и привязку
роли, вы можете задать параметр `kubernetes_role_name`.

```shell-session
$ stronghold write kubernetes/roles/auto-managed-sa-role \
    allowed_kubernetes_namespaces="test" \
    kubernetes_role_name="test-role-list-pods"
```

{% alert %}
Учетной записи сервиса Stronghold также потребуется доступ к ресурсам, к которым она предоставляет доступ.
Это можно сделать для приведенных выше примеров с помощью команды `d8 k -n test create rolebinding --role test-role-list-pods --serviceaccount=stronghold:stronghold stronghold stronghold-test-role-abilities`.
Так Kubernetes предотвращает эскалацию привилегий. Более подробную информацию вы можете прочитать в документации к [Kubernetes RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#privilege-escalation-prevention-and-bootstrapping).
{% endalert %}

После чего вы можете получить учетные данные с помощью автоматически созданной учетной записи сервиса.

```shell-session
$ stronghold write kubernetes/creds/auto-managed-sa-role \
    kubernetes_namespace=test
Key                          Value
---                          -----
lease_id                     kubernetes/creds/auto-managed-sa-role/cujRLYjKZUMQk6dkHBGGWm67
lease_duration               768h
lease_renewable              false
service_account_name         v-token-auto-man-1653001548-5z6hrgsxnmzncxejztml4arz
service_account_namespace    test
service_account_token        eyJHbGci0iJSUzI1Ni...
```

Кроме того, Stronghold может автоматически создать роль в дополнение к учетной записи сервиса и
привязке роли, указав параметр `generated_role_rules`, в который передается набор правил
JSON или YAML для создаваемой роли.

```shell-session
$ stronghold write kubernetes/roles/auto-managed-sa-and-role \
    allowed_kubernetes_namespaces="test" \
    generated_role_rules='{"rules":[{"apiGroups":[""],"resources":["pods"],"verbs":["list"]}]}'
```

После этого можно получить учетные данные тем же способом, что и раньше.

```shell-session
$ stronghold write kubernetes/creds/auto-managed-sa-and-role \
    kubernetes_namespace=test
Key                          Value
---                          -----
lease_id                     kubernetes/creds/auto-managed-sa-and-role/pehLtegoTP8vCkcaQozUqOHf
lease_duration               768h
lease_renewable              false
service_account_name         v-token-auto-man-1653002096-4imxf3ytjh5hbyro9s1oqdo3
service_account_namespace    test
service_account_token        eyJHbGci0iJSUzI1Ni...
```
