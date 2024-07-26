---
title: "Модуль user-authz"
---

Модуль отвечает за генерацию объектов ролевой модели доступа.

{% alert level="warning" %}
С версии Deckhouse Kubernetes Platform v1.63 в модуле реализована новая модель ролевого доступа. Старая модель ролевого доступа продолжит работать, но в будущем перестанет поддерживаться.

Функциональность старой и новой модели ролевого доступа не совместимы. Автоматическая конвертация ресурсов невозможна.
{% endalert %}

{% alert level="warning" %}
Документация модуля подразумевает использование [новой ролевой модели](#новая-ролевая-модель), если не указано иное.
{% endalert %}

Модуль реализует ролевую модель доступа на базе стандартного механизма RBAC Kubernetes. Создает набор кластерных ролей (_ClusterRole_), подходящий для большинства задач по управлению доступом пользователей и групп.

## Новая ролевая модель

В отличие от [устаревшей ролевой модели](#устаревшая-ролевая-модель) DKP, новая ролевая модель не использует ресурсы _ClusterAuthorizationRule_ и _AuthorizationRule_. Вся настройка прав доступа выполняется стандартным для RBAC Kubernetes путем, с использованием ресурсов _Role_, _RoleBinding_, _ClusterRole_, _ClusterRoleBinding_ и _ServceAccount_.

Для реализации новой ролевой модели DKP, модуль создает специальные агрегированные кластерные роли (_ClusterRole_). Используя эти роли в _RoleBinding_ или _ClusterRoleBinding_ можно дополнительно к стандартным задачам по управлению доступом решать следующие задачи:
- Управлять доступом к модулям из определенной [области](#области-ролевой-модели).

  Например, чтобы дать возможность настройки _сетевых_ модулей, можно использовать в _ClusterRoleBinding_ роль `d8:manage:networking:manager`. 
- Управлять доступом на настройку ресурсов модулей (но не самих модулей) в рамках namespace.

  Например, использование роли `d8:use:role:manager` в _RoleBinding_, позволит управлять ресурсом _PodLoggingConfig_, но не _ClusterLoggingConfig_ и ClusterLogDestination модуля log-shipper, а также не даст возможность настраивать сам модуль log-shipper.

Актуальность кластерных ролей, создаваемых модулем, поддерживается автоматически — необходимые права обновляются при включении/отключении модулей.  

Существует два класса ролей:
- **Manage-роль** — распространяет права на управление модулями (ресурсы _moduleConfig_) и их clusterwide-ресурсами.

  Формат названия manage-роли — `d8:manage:<SCOPE>:<ACCESS_LEVEL>`, где:
  - `SCOPE` — область действия. Может быть либо `all`, для доступа в рамках всего кластера, либо одной областей из [списка](#области-ролевой-модели).
     
    Обратите внимание, что при использовании области действия `all`, кроме прав на доступ к конфигурации модулей (ресурсы _moduleConfig_) и их clusterwide-ресурсам, в роль включена права на доступ к namespaced-ресурсам модулей и объектам Kubernetes.
  - `ACCESS_LEVEL` — [уровень доступа](#уровни-доступа-ролевой-модели).

  Примеры manage-ролей:
  - `d8:manage:all:user` — доступ на уровне `User` в рамках всего кластера (при использовании в _ClusterRoleBinding_) на управление модулями (ресурсы _moduleConfig_), их clusterwide-ресурсами, их namespaced-ресурсами и объектами Kubernetes. При использовании роли в _RoleBinding_, доступ ограничен только namespaced-ресурсами модулей;
  - `d8:manage:all:admin` — аналогично роли `d8:manage:all:user`, только доступ на уровне `Admin`;
  - `d8:manage:observability:user` — доступ на уровне `User` к clusterwide-ресурсам и конфигурации модулей (ресурсы _moduleConfig_) из области `observability`. В этом случае в роль **не входят** права на доступ к namespaced-ресурсам модулей и стандартным объектам Kubernetes;

- **Use-роль** — распространяет права на использование namespaced-ресурсов модулей и стандартных namespaced-объектов Kubernetes (Pod, Deployment, Secret, ConfigMap и т. п.). **Может использоваться только в _RoleBinding_.**  

  Формат названия use-роли — `d8:use:role:<ACCESS_LEVEL>`, где:
  - `ACCESS_LEVEL` — [уровень доступа](#уровни-доступа-ролевой-модели).
  Примеры use-ролей, доступных для использования в _RoleBinding_:
  - `d8:use:role:user` — доступ на уровне `User` на использование namespaced-ресурсов модулей и стандартных namespaced-объектов Kubernetes в рамках namespace, указанного в _RoleBinding_;
  - `d8:use:role:admin` — аналогично роли `d8:use:role:user`, только доступ на уровне `Admin`;

### Уровни доступа ролевой модели

В ролевой модели предусмотрены следующие уровни доступа (в порядке увеличения количества прав):
- `guest` — права на проверку текущих атрибутов пользователя, таких как: имя, состав групп и т.п. Позволяет выполнять `kubectl auth whoami`;
- `viewer` — дополнительно к роли `guest` позволяет просматривать стандартные ресурсы Kubernetes (кроме секретов и ресурсов RBAC).
- `user` — дополнительно к роли `viewer` позволяет просматривать секреты, подключаться к подам, удалять поды, выполнять `kubectl port-forward` и `kubectl proxy`, изменять количество реплик контроллеров; 
- `manager` — дополнительно к роли `user` позволяет управлять ресурсами модулей (например, _Certificate_, _PodLoggingConfig_ и т. п.);
- `admin` — роль с наибольшими правами.

### Области ролевой модели

Каждый внутренний модуль DKP принадлежит определенной области. Для каждой области существует набор ролей для каждого уровня доступа, кроме уровня `guest`.

Например, для области `networking` существуют следующие роли, которые можно использовать в _ClusterRoleBinding_: 
- `d8:manage:networking:viewer`
- `d8:manage:networking:user`
- `d8:manage:networking:manager`
- `d8:manage:networking:admin`

Область роли ограничивает ее действие всем кластером (область `all`) или определенным набором модулей (см. таблицу состава областей).

> Область `all`, в отличие от остальных областей роли, включает управление доступом к namespaced-ресурсам модулей и объектам Kubernetes. Область `all` может управлять доступом:
> - к конфигурации модулей (ресурсы _moduleConfig_);
> - к clusterwide-ресурсам модулей;
> - к namespaced-ресурсам модулей;
> - к объектам Kubernetes.

Таблица состава областей, используемых при генерации scope-ролей:

{% include rbac-scopes-list.liquid %}

## Устаревшая ролевая модель

Особенности:
- Реализует role-based-подсистему сквозной авторизации, расширяя функционал стандартного механизма RBAC.
- Настройка прав доступа происходит с помощью [ресурсов](cr.html).
- Управление доступом к инструментам масштабирования (параметр `allowScale` ресурса [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule-v1-spec-allowscale) или [AuthorizationRule](cr.html#authorizationrule-v1alpha1-spec-allowscale)).
- Управление доступом к форвардингу портов (параметр `portForwarding` ресурса [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule-v1-spec-portforwarding) или [AuthorizationRule](cr.html#authorizationrule-v1alpha1-spec-portforwarding)).
- Управление списком разрешенных namespace в формате labelSelector (параметр `namespaceSelector` custom resource [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule-v1-spec-namespaceselector)).

В модуле, кроме прямого использования RBAC, можно использовать удобный набор высокоуровневых ролей:
- `User` — позволяет получать информацию обо всех объектах (включая доступ к журналам подов), но не позволяет заходить в контейнеры, читать Secret'ы и выполнять port-forward;
- `PrivilegedUser` — то же самое, что и `User`, но позволяет заходить в контейнеры, читать Secret'ы, а также удалять поды (что обеспечивает возможность перезагрузки);
- `Editor` — то же самое, что и `PrivilegedUser`, но предоставляет возможность создавать, изменять и удалять все объекты, которые обычно нужны для прикладных задач;
- `Admin` — то же самое, что и `Editor`, но позволяет удалять служебные объекты (производные ресурсы, например `ReplicaSet`, `certmanager.k8s.io/challenges` и `certmanager.k8s.io/orders`);
- `ClusterEditor` — то же самое, что и `Editor`, но позволяет управлять ограниченным набором `cluster-wide`-объектов, которые могут понадобиться для прикладных задач (`ClusterXXXMetric`, `KeepalivedInstance`, `DaemonSet` и т. д). Роль для работы оператора кластера;
- `ClusterAdmin` — то же самое, что и `ClusterEditor` + `Admin`, но позволяет управлять служебными `cluster-wide`-объектами (производные ресурсы, например `MachineSets`, `Machines`, `OpenstackInstanceClasses` и т. п., а также `ClusterAuthorizationRule`, `ClusterRoleBindings` и `ClusterRole`). Роль для работы администратора кластера. **Важно**, что `ClusterAdmin`, поскольку он уполномочен редактировать `ClusterRoleBindings`, может **сам себе расширить полномочия**;
- `SuperAdmin` — разрешены любые действия с любыми объектами, при этом ограничения [`namespaceSelector`](#возможности-модуля) и [`limitNamespaces`](#возможности-модуля) продолжат работать.

> **Важно!** Режим multi-tenancy (авторизация по namespace) в данный момент реализован по временной схеме и **не гарантирует безопасность!**

В случае, если в [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule)-ресурсе используется `namespaceSelector`, `limitNamespaces` и `allowAccessToSystemNamespace` параметры не учитываются.

Если webhook, который реализовывает систему авторизации, по какой-то причине будет недоступен, в это время опции `allowAccessToSystemNamespaces`, `namespaceSelector` и `limitNamespaces` в custom resource перестанут применяться и пользователи будут иметь доступ во все namespace. После восстановления доступности webhook'а опции продолжат работать.

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

Вы можете получить дополнительный список правил доступа для роли модуля из кластера ([существующие пользовательские правила](usage.html#настройка-прав-высокоуровневых-ролей) и нестандартные правила из других модулей Deckhouse):

```bash
D8_ROLE_NAME=Editor
kubectl get clusterrole -A -o jsonpath="{range .items[?(@.metadata.annotations.user-authz\.deckhouse\.io/access-level=='$D8_ROLE_NAME')]}{.rules}{'\n'}{end}" | jq -s add
```
