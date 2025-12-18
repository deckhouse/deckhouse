---
title: "Текущая модель авторизации"
permalink: ru/admin/configuration/access/authorization/rbac-current.html
description: "Настройка текущей модели RBAC авторизации в Deckhouse Kubernetes Platform. Настройка модуля user-authz, управление ClusterRole и конфигурация ролевого доступа."
lang: ru
---

Для реализации текущей ролевой модели в кластере должен быть включён модуль [`user-authz`](/modules/user-authz/).
Модуль создаёт набор кластерных ролей (ClusterRole), подходящий для большинства задач по управлению доступом пользователей и групп.

{% alert level="warning" %} С версии Deckhouse Kubernetes Platform v1.64 в модуле реализована экспериментальная модель ролевого доступа. Текущая модель ролевого доступа продолжит работать, но в будущем будет объявлена устаревшей (deprecated).

Функциональности экспериментальной и текущей моделей ролевого доступа несовместимы. Автоматическая конвертация ресурсов невозможна.
{% endalert %}

<!-- Перенесено с некоторыми изменениям из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/user-authz/#%D1%82%D0%B5%D0%BA%D1%83%D1%89%D0%B0%D1%8F-%D1%80%D0%BE%D0%BB%D0%B5%D0%B2%D0%B0%D1%8F-%D0%BC%D0%BE%D0%B4%D0%B5%D0%BB%D1%8C -->

Особенности текущей ролевой модели:

- Реализует role-based-подсистему сквозной авторизации, расширяя функционал стандартного механизма RBAC.
- Настройка прав доступа происходит с помощью кастомных ресурсов [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) и [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule).
- Управление доступом к инструментам масштабирования (параметр `allowScale` ресурса [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-allowscale) или [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule-v1alpha1-spec-allowscale)).
- Управление доступом к форвардингу портов (параметр `portForwarding` ресурса [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-portforwarding) или [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule-v1alpha1-spec-portforwarding)).
- Управление списком разрешённых пространств имён в формате labelSelector (параметр `namespaceSelector` ресурса [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-namespaceselector)).

## Высокоуровневые роли, используемые для реализации модели

Для реализации текущей ролевой модели с помощью модуля [`user-authz`](/modules/user-authz/), кроме использования RBAC, можно использовать удобный набор высокоуровневых ролей:

| Роль             | Примеры доступных действий                                                                                                              | Ограничения                                  |
|------------------|-----------------------------------------------------------------------------------------------------------------------------------------|---------------------------------------------|
| **User**         | Просмотр подов, логов, Deployment                                                                                                   | Нет доступа к секретам, портам, контейнерам |
| **PrivilegedUser** | Вход в контейнеры (`kubectl exec`), чтение секретов, удаление подов                                                                     | Не может изменять Deployment/Service        |
| **Editor**       | Создание/удаление Deployment, Service, ConfigMap                                                                                        | Нет доступа к ReplicaSet, ClusterRoles  |
| **Admin**        | Удаление ReplicaSet, управление RBAC в пространстве имён                                                                                      | Нет доступа к ресурсам на уровне кластера         |
| **ClusterEditor** | Создание DaemonSet, ClusterRole, ClusterXXXMetric, KeepalivedInstance (только тех, что могут понадобиться для прикладных задач) | Не может удалять MachineSets              |
| **ClusterAdmin** | Полный доступ к ClusterRoleBindings, Machines, OpenstackInstanceClasses                                                           | Может повысить свои права                   |
| **SuperAdmin**   | Любые действия (включая `*` в RBAC), но с учетом `limitNamespaces`                                                                      | Ограничения только через политики кластера  |

{% alert level="warning" %}
Режим multitenancy (авторизация по пространству имён) в данный момент реализован по временной схеме и **не гарантирует безопасность**.
{% endalert %}

В случае, если в ресурсе [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) используется `namespaceSelector`, параметры `limitNamespaces` и `allowAccessToSystemNamespace` не учитываются.

Если вебхук, который реализовывает систему авторизации, по какой-то причине будет недоступен, опции `allowAccessToSystemNamespaces`, `namespaceSelector` и `limitNamespaces` в кастомных ресурсах перестанут применяться и пользователи будут иметь доступ во все пространства имён. После восстановления доступности вебхука опции продолжат работать.

## Список доступа для каждой высокоуровневой роли по умолчанию

Сокращения для `verbs`:
<!-- start user-authz roles placeholder -->
* read — `get`, `list`, `watch`;
* read-write — `get`, `list`, `watch`, `create`, `delete`, `deletecollection`, `patch`, `update`;
* write — `create`, `delete`, `deletecollection`, `patch`, `update`.

{{site.data.i18n.common.role[page.lang] | capitalize }} `User`:

```text
read:
    - apiextensions.k8s.io/customresourcedefinitions
    - apps/daemonsets
    - apps/deployments
    - apps/replicasets
    - apps/statefulsets
    - autoscaling.k8s.io/verticalpodautoscalers
    - autoscaling/horizontalpodautoscalers
    - batch/cronjobs
    - batch/jobs
    - configmaps
    - discovery.k8s.io/endpointslices
    - endpoints
    - events
    - events.k8s.io/events
    - extensions/daemonsets
    - extensions/deployments
    - extensions/ingresses
    - extensions/replicasets
    - extensions/replicationcontrollers
    - limitranges
    - metrics.k8s.io/nodes
    - metrics.k8s.io/pods
    - namespaces
    - networking.k8s.io/ingresses
    - networking.k8s.io/networkpolicies
    - nodes
    - persistentvolumeclaims
    - persistentvolumes
    - pods
    - pods/log
    - policy/poddisruptionbudgets
    - rbac.authorization.k8s.io/rolebindings
    - rbac.authorization.k8s.io/roles
    - replicationcontrollers
    - resourcequotas
    - serviceaccounts
    - services
    - storage.k8s.io/storageclasses
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `PrivilegedUser` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`):

```text
create:
    - pods/eviction
create,get:
    - pods/attach
    - pods/exec
delete,deletecollection:
    - pods
read:
    - secrets
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `Editor` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`):

```text
read-write:
    - apps/deployments
    - apps/statefulsets
    - autoscaling.k8s.io/verticalpodautoscalers
    - autoscaling/horizontalpodautoscalers
    - batch/cronjobs
    - batch/jobs
    - configmaps
    - discovery.k8s.io/endpointslices
    - endpoints
    - extensions/deployments
    - extensions/ingresses
    - networking.k8s.io/ingresses
    - persistentvolumeclaims
    - policy/poddisruptionbudgets
    - serviceaccounts
    - services
write:
    - secrets
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `Admin` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`):

```text
create,patch,update:
    - pods
delete,deletecollection:
    - apps/replicasets
    - extensions/replicasets
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `ClusterEditor` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`):

```text
read:
    - rbac.authorization.k8s.io/clusterrolebindings
    - rbac.authorization.k8s.io/clusterroles
write:
    - apiextensions.k8s.io/customresourcedefinitions
    - apps/daemonsets
    - extensions/daemonsets
    - storage.k8s.io/storageclasses
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `ClusterAdmin` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`):

```text
read-write:
    - deckhouse.io/clusterauthorizationrules
write:
    - limitranges
    - namespaces
    - networking.k8s.io/networkpolicies
    - rbac.authorization.k8s.io/clusterrolebindings
    - rbac.authorization.k8s.io/clusterroles
    - rbac.authorization.k8s.io/rolebindings
    - rbac.authorization.k8s.io/roles
    - resourcequotas
```
<!-- end user-authz roles placeholder -->

Вы можете получить дополнительный список правил доступа для роли модуля из кластера ([существующие пользовательские правила](granting.html#предоставление-прав-с-помощью-authorizationrule-и-clusterauthorizationrule-текущая-ролевая-модель) и нестандартные правила из других модулей Deckhouse) с помощью команды:

```bash
D8_ROLE_NAME=Editor
d8 k get clusterrole -A -o jsonpath="{range .items[?(@.metadata.annotations.user-authz\.deckhouse\.io/access-level=='$D8_ROLE_NAME')]}{.rules}{'\n'}{end}" | jq -s add
```

## Пример AuthorizationRule

Используйте [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) для установки правил доступа для пользователей внутри определённого пространства имен.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: AuthorizationRule
metadata:
  name: beeline
spec:
  accessLevel: Admin
  subjects:
  - kind: Admin
    name: admin@example.com
```

## Пример ClusterAuthorizationRule

[ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) можно использовать для установки правил доступа для пользователей как на уровне всего кластера, так и на уровне определенных пространств имен.

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: test-rule
spec:
  subjects:
  - kind: User
    name: some@example.com
  - kind: ServiceAccount
    name: gitlab-runner-deploy
    namespace: d8-service-accounts
  - kind: Group
    name: some-group-name
  accessLevel: PrivilegedUser
  portForwarding: true
  # Опция доступна только при включенном режиме enableMultiTenancy (версия Enterprise Edition).
  allowAccessToSystemNamespaces: false
  # Опция доступна только при включенном режиме enableMultiTenancy (версия Enterprise Edition).
  namespaceSelector:
    labelSelector:
      matchExpressions:
      - key: stage
        operator: In
        values:
        - test
        - review
      matchLabels:
        team: frontend
```

## Расширение прав доступа для высокоуровневых ролей

Если требуется добавить права для определённой [высокоуровневой роли](../authorization/rbac-current.html#высокоуровневые-роли-используемые-для-реализации-модели), создайте ClusterRole с аннотацией `user-authz.deckhouse.io/access-level: <AccessLevel>`.

Пример:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  annotations:
    user-authz.deckhouse.io/access-level: Editor
  name: user-editor
rules:
- apiGroups:
  - kuma.io
  resources:
  - trafficroutes
  - trafficroutes/finalizers
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - flagger.app
  resources:
  - canaries
  - canaries/status
  - metrictemplates
  - metrictemplates/status
  - alertproviders
  - alertproviders/status
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
```
