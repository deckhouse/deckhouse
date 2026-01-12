---
title: "Проверка наличия прав у пользователя"
permalink: ru/admin/configuration/access/authorization/check.html
description: "Проверка прав доступа пользователей в Deckhouse Kubernetes Platform. Верификация RBAC разрешений, тестирование доступа пользователей и инструменты отладки авторизации."
lang: ru
---

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

Если в кластере включён режим **multitenancy**, выполните ещё одну проверку, чтобы убедиться, что у пользователя есть доступ в пространство имён:

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
