---
title: "Проверка наличия прав у пользователя"
permalink: ru/admin/configuration/access/authorization/check.html
description: "Проверка прав доступа пользователей в Deckhouse Platform. Верификация RBAC разрешений, тестирование доступа пользователей и инструменты отладки авторизации."
lang: ru
---

## Быстрая проверка одного права

Быстрый способ проверить одно конкретное право — использование команды `d8 k auth can-i`:

```shell
d8 k auth can-i --as=user@example.com get pods -n other-namespace
```

Чтобы получить полную картину того, что разрешено пользователю, группе или ServiceAccount — а не только один глагол над одним ресурсом, — используйте ресурс [SubjectAccessReport](#получение-полного-отчёта-о-правах), описанный ниже.

## Проверка наличия доступа у пользователя

Чтобы проверить наличие прав доступа у пользователя, выполните следующую команду, в которой будут указаны:

* `resourceAttributes` (как в RBAC) — к чему проверяется доступ;
* `user` — имя пользователя;
* `groups` — группы пользователя.

{% alert level="info" %}
При совместном использовании с [модулем `user-authn`](/modules/user-authn/) группы и имя пользователя можно посмотреть в логах Dex с помощью команды `d8 k -n d8-user-authn logs -l app=dex` (видны только при авторизации).
{% endalert %}

```shell
cat  <<EOF | 2>&1 d8 k create --raw  /apis/authorization.k8s.io/v1/subjectaccessreviews -f - | jq .status
{
  "apiVersion": "authorization.k8s.io/v1",
  "kind": "SubjectAccessReview",
  "spec": {
    "resourceAttributes": {
      "namespace": "",
      "verb": "watch",
      "version": "v1",
      "resource": "pods"
    },
    "user": "system:kube-controller-manager",
    "groups": [
      "Admins"
    ]
  }
}
EOF
```

В результате будет видно, есть ли доступ и на основании какой роли.

Пример ответа при наличии прав доступа у пользователя:

```json
{
  "allowed": true,
  "reason": "RBAC: allowed by ClusterRoleBinding \"system:kube-controller-manager\" of ClusterRole \"system:kube-controller-manager\" to User \"system:kube-controller-manager\""
}
```

Пример ответа при отсутствии прав доступа у пользователя:

```json
{
  "allowed": false
}
```

Проверки выше касаются только гранулярной ролевой модели, которую напрямую применяет RBAC. Если в кластере включён режим **multitenancy** и дополнительно используется [упрощённая ролевая модель](/modules/user-authz/#упрощённая-ролевая-модель) (ClusterAuthorizationRule/AuthorizationRule), её ограничения по неймспейсам применяются отдельным вебхуком авторизации — выполните по нему дополнительную проверку, чтобы убедиться, что у пользователя есть доступ в неймспейс:

```shell
cat  <<EOF | 2>&1 d8 k --kubeconfig /etc/kubernetes/deckhouse/extra-files/webhook-config.yaml create --raw / -f - | jq .status
{
  "apiVersion": "authorization.k8s.io/v1",
  "kind": "SubjectAccessReview",
  "spec": {
    "resourceAttributes": {
      "namespace": "",
      "verb": "watch",
      "version": "v1",
      "resource": "pods"
    },
    "user": "system:kube-controller-manager",
    "groups": [
      "Admins"
    ]
  }
}
EOF
```

Пример ответа при наличии прав доступа у пользователя:

```json
{
  "allowed": false
}
```

Сообщение `"allowed": false` значит, что вебхук не блокирует запрос. В случае блокировки запроса вебхуком вы получите, например, следующее сообщение:

```json
{
  "allowed": false,
  "denied": true,
  "reason": "making cluster scoped requests for namespaced resources are not allowed"
}
```

## Получение полного отчёта о правах

При включённом режиме мультитенантности ([`enableMultiTenancy`](/modules/user-authz/configuration.html#parameters-enablemultitenancy)) ресурс SubjectAccessReport модуля `user-authz` сразу возвращает готовый отчёт обо всём, что разрешено субъекту, вместо проверки прав по одному действию за раз. В отчете содержатся сведения о том, какие роли и через какие привязки выданы, какие действия и над какими ресурсами разрешены в кластере и в каждом неймспейсе, и откуда взялось каждое право.

Пример получения отчета:

```shell
d8 k create -o yaml -f - <<EOF
apiVersion: authorization.deckhouse.io/v1alpha1
kind: SubjectAccessReport
metadata:
  name: what-can-jane-do
spec:
  subject:
    kind: User
    name: jane@example.com
EOF
```

Поле `spec.subject.kind` принимает значения `User`, `Group` и `ServiceAccount` (для последнего обязательно укажите `spec.subject.namespace`). Если `spec.subject` не указан, отчёт строится для того, кто выполняет запрос.

Отчёт о самом себе доступен любому аутентифицированному пользователю и не требует дополнительных прав. Для построения отчёта о **другом** субъекте нужна кластерная роль `d8:user-authz:subject-access-checker`, которая никому не выдана по умолчанию.

Есть и обратный запрос — ресурс `WhoCan` — он отвечает на вопрос «кто может выполнить действие X над ресурсом Y?» и возвращает список пользователей, групп и ServiceAccount'ов. Право создавать запросы `WhoCan` даёт кластерная роль `d8:user-authz:who-can-checker`. Подробнее об обоих ресурсах — в [FAQ модуля `user-authz`](/modules/user-authz/faq.html#как-узнать-что-разрешено-конкретному-пользователю-группе-или-serviceaccountу).
