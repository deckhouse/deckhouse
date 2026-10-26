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

Чтобы получить полный отчёт о правах пользователя, группы или ServiceAccount, а не только проверить отдельное действие над отдельным ресурсом, используйте описанный ниже ресурс [SubjectAccessReport](#получение-полного-отчёта-о-правах).

## Проверка наличия доступа у пользователя

Чтобы проверить наличие прав доступа у пользователя, выполните следующую команду, в которой будут указаны:

* `resourceAttributes` (как в RBAC) — к чему проверяется доступ;
* `user` — имя пользователя;
* `groups` — группы пользователя.

{% alert level="info" %}
При совместном использовании с [модулем `user-authn`](/modules/user-authn/) группы и имя пользователя можно посмотреть в логах Dex с помощью команды `d8 k -n d8-user-authn logs -l app=dex`. Они появляются в логах только после входа пользователя.
{% endalert %}

```shell
cat  <<EOF | 2>&1 d8 k create --raw  /apis/authorization.k8s.io/v1/subjectaccessreviews -f - | jq .status
{
  "apiVersion": "authorization.k8s.io/v1",
  "kind": "SubjectAccessReview",
  "spec": {
    "resourceAttributes": {
      "namespace": "dev",
      "verb": "watch",
      "version": "v1",
      "resource": "pods"
    },
    "user": "user@example.com",
    "groups": [
      "developers"
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
  "reason": "RBAC: allowed by RoleBinding \"<BINDING_NAME>/dev\" of ClusterRole \"<ROLE_NAME>\" to User \"user@example.com\""
}
```

Пример ответа при отсутствии прав доступа у пользователя:

```json
{
  "allowed": false
}
```

В режиме мультитенантности ограничения по неймспейсам [упрощённой ролевой модели](rbac-current.html) (ClusterAuthorizationRule/AuthorizationRule) применяет вебхук авторизации. Ответ API-сервера уже учитывает решение вебхука. Чтобы получить ответ только вебхука, выполните на master-узле от имени root следующую команду:

```shell
cat  <<EOF | 2>&1 d8 k --kubeconfig /etc/kubernetes/deckhouse/extra-files/webhook-config.yaml create --raw / -f - | jq .status
{
  "apiVersion": "authorization.k8s.io/v1",
  "kind": "SubjectAccessReview",
  "spec": {
    "resourceAttributes": {
      "namespace": "dev",
      "verb": "watch",
      "version": "v1",
      "resource": "pods"
    },
    "user": "user@example.com",
    "groups": [
      "developers"
    ]
  }
}
EOF
```

Пример ответа при отсутствии прав доступа у пользователя:

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
  "reason": "either you have no access to the namespace or the namespace does not exist"
}
```

## Получение полного отчёта о правах

При включённом режиме мультитенантности ([`enableMultiTenancy`](/modules/user-authz/configuration.html#parameters-enablemultitenancy)) ресурс SubjectAccessReport модуля `user-authz` возвращает полный отчёт о правах субъекта, а не проверяет права для каждого действия отдельно. В отчёте содержатся сведения о том, какие роли и через какие привязки назначены субъекту, какие действия над какими ресурсами разрешены в кластере и каждом неймспейсе, а также откуда получено каждое право.

Пример получения отчёта:

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

Поле `spec.subject.kind` принимает значения `User`, `Group` и `ServiceAccount` (для последнего обязательно укажите `spec.subject.namespace`). Если `spec.subject` не указан, отчёт формируется для того, кто выполняет запрос.

Отчёт о самом себе доступен любому аутентифицированному пользователю и не требует дополнительных прав. Для построения отчёта о **другом** субъекте нужна кластерная роль `d8:user-authz:subject-access-checker`, которая никому не выдана по умолчанию.

Есть и обратный запрос — ресурс `WhoCan` — он отвечает на вопрос «кто может выполнить действие X над ресурсом Y?» и возвращает список пользователей, групп и ServiceAccount'ов. Пример:

```yaml
d8 k create -o yaml -f - <<EOF
apiVersion: authorization.deckhouse.io/v1alpha1
kind: WhoCan
metadata:
  name: who-can-create-networkpolicies
spec:
  resourceAttributes:
    namespace: my-namespace
    verb: create
    group: networking.k8s.io
    resource: networkpolicies
EOF
```

Ответ возвращается в поле `status` (содержит поля `users`, `groups`, `serviceAccounts`) сразу в выводе команды. Объект нигде не сохраняется.

Право создавать запросы `WhoCan` даёт кластерная роль `d8:user-authz:who-can-checker`. Она никому не выдана по умолчанию: результат запроса раскрывает субъекты доступа во всех неймспейсах, поэтому выдавайте её только доверенным администраторам через ClusterRoleBinding.

Строить отчёты о других субъектах и создавать запросы `WhoCan` могут и менеджеры подсистем `iam` и `security`. Это право даёт capability `d8:subsystem-capability:iam:inspect_access` ролей `d8:subsystem:iam:manager` и `d8:subsystem:security:manager`, поэтому оно есть и у ролей выше них (`d8:subsystem:iam:superadmin`, `d8:subsystem:security:superadmin`, `d8:system:manager` и `d8:system:superadmin`). Подробнее об обоих ресурсах — в [FAQ модуля `user-authz`](/modules/user-authz/faq.html#как-узнать-что-разрешено-конкретному-пользователю-группе-или-serviceaccountу).
