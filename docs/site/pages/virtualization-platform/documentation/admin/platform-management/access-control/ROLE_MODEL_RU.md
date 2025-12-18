---
title: "Ролевая модель"
permalink: ru/virtualization-platform/documentation/admin/platform-management/access-control/role-model.html
lang: ru
---

## Описание

Deckhouse Virtualization Platform (DVP) предоставляет стандартный набор ролей для управления доступом к проектным и кластерным ресурсам, которые разделены на два типа:

- [Use-роли](#use-роли) — эти роли назначаются пользователям проекта и позволяют им управлять ресурсами в рамках **указанного проекта**.
- [Manage-роли](#manage-роли) — эти роли предназначены для администраторов DVP, предоставляя им права на управление ресурсами на уровне всей платформы.

Права доступа в DVP настраиваются с использованием стандартного подхода RBAC Kubernetes, что предполагает создание ресурсов `RoleBinding` или `ClusterRoleBinding`, в которых указывается соответствующая роль

### Use-роли

{% alert level="warning" %}
Use-роль можно использовать только в ресурсе `RoleBinding`.
{% endalert %}

Use-роли предназначены для назначения прав пользователю **в конкретном пространстве имён**. Под пользователями понимаются, например, разработчики, которые используют настроенный администратором кластер для развёртывания своих приложений. Таким пользователям не нужно управлять модулями DVP или кластером, но им нужно иметь возможность, например, создавать свои Ingress-ресурсы, настраивать аутентификацию приложений и сбор логов с приложений.

Use-роль определяет права на доступ к namespaced-ресурсам модулей и стандартным namespaced-ресурсам Kubernetes (`Pod`, `Deployment`, `Secret`, `ConfigMap` и т. п.).

Модуль создаёт следующие use-роли:

- `d8:use:role:viewer` — позволяет в конкретном пространстве имён просматривать стандартные ресурсы Kubernetes, кроме секретов и ресурсов RBAC, а также выполнять аутентификацию в кластере;
- `d8:use:role:user` — дополнительно к роли `d8:use:role:viewer` позволяет в конкретном пространстве имён просматривать секреты и ресурсы RBAC, подключаться к подам, удалять поды (но не создавать или изменять их), выполнять `kubectl port-forward` и `kubectl proxy`, изменять количество реплик контроллеров;
- `d8:use:role:manager` — дополнительно к роли `d8:use:role:user` позволяет в конкретном пространстве имён управлять ресурсами модулей (например, `Certificate`, `PodLoggingConfig` и т. п.) и стандартными namespaced-ресурсами Kubernetes (`Pod`, `ConfigMap`, `CronJob` и т. п.);
- `d8:use:role:admin` — дополнительно к роли `d8:use:role:manager` позволяет в конкретном пространстве имён управлять ресурсами `ResourceQuota`, `ServiceAccount`, `Role`, `RoleBinding`, `NetworkPolicy`.

### Manage-роли

{% alert level="warning" %}
Manage-роль не дает доступа к пространству имён пользовательских приложений.

Manage-роль определяет доступ только к системным пространствам имён (начинающимся с `d8-` или `kube-`), и только к тем из них, в которых работают модули соответствующей подсистемы роли.
{% endalert %}

Manage-роли предназначены для назначения прав на управление DVP или её частью ([подсистемой](#подсистемы-ролевой-модели)), но не самими приложениями пользователей. С помощью manage-роли можно, например, дать возможность администратору безопасности управлять модулями, ответственными за функции безопасности кластера. Тогда администратор безопасности сможет настраивать аутентификацию, авторизацию, политики безопасности и т. п., но не сможет управлять остальными функциями кластера (например, настройками сети и мониторинга) и изменять настройки в пространстве имён приложений пользователей.

Manage-роль определяет права на доступ:

- к cluster-wide-ресурсам Kubernetes;
- к управлению модулями DVP (ресурсы `moduleConfig`) в рамках [подсистемы](#подсистемы-ролевой-модели) роли, или всеми модулями DVP для роли `d8:manage:all:*`;
- к управлению cluster-wide-ресурсами модулей DVP в рамках [подсистемы](#подсистемы-ролевой-модели) роли или всеми ресурсами модулей DVP для роли `d8:manage:all:*`;
- к системным пространствам имён (начинающимся с `d8-` или `kube-`), в которых работают модули [подсистемы](#подсистемы-ролевой-модели) роли, или ко всем системным пространствам имён для роли `d8:manage:all:*`.

Формат названия manage-роли — `d8:manage:<SUBSYSTEM>:<ACCESS_LEVEL>`, где:

- `SUBSYSTEM` — подсистема роли. Может быть либо одной из подсистем [списка](#подсистемы-ролевой-модели), либо `all` для доступа в рамках всех подсистем;
- `ACCESS_LEVEL` — уровень доступа.

  Примеры manage-ролей:

  - `d8:manage:all:viewer` — доступ на просмотр конфигурации всех модулей DVP (ресурсы `moduleConfig`), их cluster-wide-ресурсов, их namespaced-ресурсов и стандартных объектов Kubernetes (кроме секретов и ресурсов RBAC) во всех системных пространствах имён (начинающихся с `d8-` или `kube-`);
  - `d8:manage:all:manager` — аналогично роли `d8:manage:all:viewer`, только доступ на уровне `admin`, т. е. просмотр/создание/изменение/удаление конфигурации всех модулей DVP (ресурсы `moduleConfig`), их cluster-wide-ресурсов, их namespaced-ресурсов и стандартных объектов Kubernetes во всех системных пространствах имён (начинающихся с `d8-` или `kube-`);
  - `d8:manage:observability:viewer` — доступ на просмотр конфигурации модулей DVP (ресурсы `moduleConfig`) из подсистемы `observability`, их cluster-wide-ресурсов, их namespaced-ресурсов и стандартных объектов Kubernetes (кроме секретов и ресурсов RBAC) в системных пространствах имён `d8-log-shipper`, `d8-monitoring`, `d8-okmeter`, `d8-operator-prometheus`, `d8-upmeter`, `kube-prometheus-pushgateway`.

Модуль предоставляет два уровня доступа для администратора:

- `viewer` — позволяет просматривать стандартные ресурсы Kubernetes, конфигурацию модулей (ресурсы `moduleConfig`), cluster-wide-ресурсы модулей и namespaced-ресурсы модулей в пространстве имен модуля;
- `manager` — дополнительно к роли `viewer` позволяет управлять стандартными ресурсами Kubernetes, конфигурацией модулей (ресурсы `moduleConfig`), cluster-wide-ресурсами модулей и namespaced-ресурсами модулей в пространстве имен модуля;

### Подсистемы ролевой модели

Каждый модуль DVP принадлежит определённой подсистемы. Для каждой подсистемы существует набор ролей с разными уровнями доступа. Роли обновляются автоматически при включении или отключении модуля.

Например, для подсистемы `networking` существуют следующие manage-роли, которые можно использовать в `ClusterRoleBinding`:

- `d8:manage:networking:viewer`
- `d8:manage:networking:manager`

Подсистема роли ограничивает её действие всеми системными (начинающимися с `d8-` или `kube-`) пространствами имён кластера (подсистема `all`) или теми пространствами имён, в которых работают модули подсистемы (см. таблицу состава подсистем).

Таблица состава подсистем ролевой модели.

{% include rbac/rbac-subsystems-list.liquid %}

<div style="height: 0;" id="устаревшая-ролевая-модель"></div>

## Текущая ролевая модель

Особенности:

- Модуль реализует role-based-подсистему сквозной авторизации, расширяя функционал стандартного механизма RBAC.
- Настройка прав доступа происходит с помощью ресурсов.
- Управление доступом к инструментам масштабирования (параметр `allowScale` ресурса [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule#clusterauthorizationrule-v1-spec-allowscale) или [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule#authorizationrule-v1alpha1-spec-allowscale)).
- Управление доступом к форвардингу портов (параметр `portForwarding` ресурса [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule#clusterauthorizationrule-v1-spec-portforwarding) или [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule#authorizationrule-v1alpha1-spec-portforwarding)).
- Управление списком разрешённых пространств имён в формате labelSelector (параметр `namespaceSelector` ресурса [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule#clusterauthorizationrule-v1-spec-namespaceselector)).

В модуле, кроме использования RBAC, можно использовать удобный набор высокоуровневых ролей:

- `User` — позволяет получать информацию обо всех объектах (включая доступ к журналам подов), но не позволяет заходить в контейнеры, читать секреты и выполнять port-forward;
- `PrivilegedUser` — то же самое, что и `User`, но позволяет заходить в контейнеры, читать секреты, а также удалять поды (что обеспечивает возможность перезагрузки);
- `Editor` — то же самое, что и `PrivilegedUser`, но предоставляет возможность создавать, изменять и удалять все объекты, которые обычно нужны для прикладных задач;
- `Admin` — то же самое, что и `Editor`, но позволяет удалять служебные объекты (производные ресурсы, например `ReplicaSet`, `certmanager.k8s.io/challenges` и `certmanager.k8s.io/orders`);
- `ClusterEditor` — то же самое, что и `Editor`, но позволяет управлять ограниченным набором `cluster-wide`-объектов, которые могут понадобиться для прикладных задач (`ClusterXXXMetric`, `KeepalivedInstance`, `DaemonSet` и т. д). Роль для работы оператора кластера;
- `ClusterAdmin` — то же самое, что и `ClusterEditor` + `Admin`, но позволяет управлять служебными `cluster-wide`-объектами (производные ресурсы, например `MachineSets`, `Machines`, `OpenstackInstanceClasses` и т. п., а также `ClusterAuthorizationRule`, `ClusterRoleBindings` и `ClusterRole`). Роль для работы администратора кластера. **Важно**, что `ClusterAdmin`, поскольку он уполномочен редактировать `ClusterRoleBindings`, может **сам себе расширить полномочия**;
- `SuperAdmin` — разрешены любые действия с любыми объектами, при этом ограничения `namespaceSelector` и `limitNamespaces` продолжат работать.

{% alert level="warning" %}
Режим multi-tenancy (авторизация по пространству имён) в данный момент реализован по временной схеме и **не гарантирует безопасность!**
{% endalert %}

В случае, если в [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) используется `namespaceSelector`, параметры `limitNamespaces` и `allowAccessToSystemNamespace` не учитываются.

Если вебхук, который реализовывает систему авторизации, по какой-то причине будет недоступен, опции `allowAccessToSystemNamespaces`, `namespaceSelector` и `limitNamespaces` в custom resource перестанут применяться и пользователи будут иметь доступ во все пространства имён. После восстановления доступности вебхука опции продолжат работать.

## Экспериментальная ролевая модель

В отличие [от текущей ролевой модели](#текущая-ролевая-модель) DVP, экспериментальная ролевая модель не использует ресурсы `ClusterAuthorizationRule` и `AuthorizationRule`. Настройка прав доступа выполняется стандартным для RBAC Kubernetes способом: с помощью создания ресурсов `RoleBinding` или `ClusterRoleBinding`, с указанием в них одной из подготовленных модулем `user-authz` ролей.

Модуль создаёт специальные агрегированные кластерные роли (`ClusterRole`). Используя эти роли в `RoleBinding` или `ClusterRoleBinding` можно решать следующие задачи:

- Управлять доступом к модулям определённой [подсистеме](#подсистемы-ролевой-модели) применения.

  Например, чтобы дать возможность пользователю, выполняющему функции сетевого администратора, настраивать *сетевые* модули (например, `cni-cilium`, `ingress-nginx`, `istio` и т. д.), можно использовать в `ClusterRoleBinding` роль `d8:manage:networking:manager`.
- Управлять доступом к *пользовательским* ресурсам модулей в рамках пространства имён.

  Например, использование роли `d8:use:role:manager` в `RoleBinding`, позволит удалять/создавать/редактировать ресурс [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig#podloggingconfig) в пространстве имён, но не даст доступа к cluster-wide-ресурсам [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig#clusterloggingconfig) и [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination#clusterlogdestination) модуля `log-shipper`, а также не даст возможность настраивать сам модуль `log-shipper`.

### Список доступа для каждой роли модуля по умолчанию

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

Вы можете получить дополнительный список правил доступа для роли из кластера существующие пользовательские правила и нестандартные правила из других модулей DVP):

```bash
D8_ROLE_NAME=Editor
d8 k get clusterrole -A -o jsonpath="{range .items[?(@.metadata.annotations.user-authz\.deckhouse\.io/access-level=='$D8_ROLE_NAME')]}{.rules}{'\n'}{end}" | jq -s add
```
