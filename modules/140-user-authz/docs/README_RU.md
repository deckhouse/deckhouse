---
title: "Модуль user-authz"
---

Модуль отвечает за создание RBAC для пользователей и реализует простейший режим multi-tenancy с разграничением доступа по namespace.

Модуль реализует role-based-подсистему сквозной авторизации, расширяя функционал стандартного механизма RBAC.

Настройка прав доступа происходит с помощью [custom resources](cr.html).

## Возможности модуля

- Управление доступом пользователей и групп на базе механизма RBAC Kubernetes.
- Управление доступом к инструментам масштабирования (параметр `allowScale` ресурса [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule-v1-spec-allowscale) или [AuthorizationRule](cr.html#authorizationrule-v1alpha1-spec-allowscale)).
- Управление доступом к форвардингу портов (параметр `portForwarding` ресурса [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule-v1-spec-portforwarding) или [AuthorizationRule](cr.html#authorizationrule-v1alpha1-spec-portforwarding)).
- Управление списком разрешенных namespace в формате labelSelector (параметр `namespaceSelector` custom resource [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule-v1-spec-namespaceselector)).

## Ролевая модель

В модуле, кроме использования RBAC, можно использовать удобный набор высокоуровневых ролей:
- `User` — позволяет получать информацию обо всех объектах (включая доступ к журналам подов), но не позволяет заходить в контейнеры, читать Secret'ы и выполнять port-forward;
- `PrivilegedUser` — то же самое, что и `User`, но позволяет заходить в контейнеры, читать Secret'ы, а также удалять поды (что обеспечивает возможность перезагрузки);
- `Editor` — то же самое, что и `PrivilegedUser`, но предоставляет возможность создавать, изменять и удалять все объекты, которые обычно нужны для прикладных задач;
- `Admin` — то же самое, что и `Editor`, но позволяет удалять служебные объекты (производные ресурсы, например `ReplicaSet`, `certmanager.k8s.io/challenges` и `certmanager.k8s.io/orders`);
- `ClusterEditor` — то же самое, что и `Editor`, но позволяет управлять ограниченным набором `cluster-wide`-объектов, которые могут понадобиться для прикладных задач (`ClusterXXXMetric`, `KeepalivedInstance`, `DaemonSet` и т. д). Роль для работы оператора кластера;
- `ClusterAdmin` — то же самое, что и `ClusterEditor` + `Admin`, но позволяет управлять служебными `cluster-wide`-объектами (производные ресурсы, например `MachineSets`, `Machines`, `OpenstackInstanceClasses` и т. п., а также `ClusterAuthorizationRule`, `ClusterRoleBindings` и `ClusterRole`). Роль для работы администратора кластера. **Важно**, что `ClusterAdmin`, поскольку он уполномочен редактировать `ClusterRoleBindings`, может **сам себе расширить полномочия**;
- `SuperAdmin` — разрешены любые действия с любыми объектами, при этом ограничения [`namespaceSelector`](#возможности-модуля) и [`limitNamespaces`](#возможности-модуля) продолжат работать.

## Особенности реализации

> **Важно!** Режим multi-tenancy (авторизация по namespace) в данный момент реализован по временной схеме и **не гарантирует безопасность!**

В случае, если в [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule)-ресурсе используется `namespaceSelector`, `limitNamespaces` и `allowAccessToSystemNamespace` параметры не учитываются.

Если webhook, который отвечает за систему авторизации, по какой-то причине недоступен, опции `allowAccessToSystemNamespaces`, `namespaceSelector` и `limitNamespaces` в custom resource перестанут применяться и пользователи получат доступ во все namespace. После восстановления доступности webhook'а опции продолжат работать.

## Список доступа для каждой роли модуля по умолчанию

Сокращения для `verbs`:
<!-- start user-authz roles placeholder -->
* read - `get`, `list`, `watch`
* read-write - `get`, `list`, `watch`, `create`, `delete`, `deletecollection`, `patch`, `update`
* write - `create`, `delete`, `deletecollection`, `patch`, `update`

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

Существует возможность получения дополнительных правил доступа для роли модуля из кластера ([существующие пользовательские правила](usage.html#настройка-прав-высокоуровневых-ролей) и получения нестандартных правил из других модулей Deckhouse):

```bash
D8_ROLE_NAME=Editor
kubectl get clusterrole -A -o jsonpath="{range .items[?(@.metadata.annotations.user-authz\.deckhouse\.io/access-level=='$D8_ROLE_NAME')]}{.rules}{'\n'}{end}" | jq -s add
```
