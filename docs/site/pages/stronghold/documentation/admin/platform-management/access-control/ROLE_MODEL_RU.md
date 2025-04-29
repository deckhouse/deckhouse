---
title: "Ролевая модель"
permalink: ru/stronghold/documentation/admin/platform-management/access-control/role-model.html
lang: ru
---

## Описание

Платформа предоставляет стандартный набор ролей для управления доступом к проектным и кластерным ресурсам, которые разделены на два типа:

- [Use-роли](#use-роли) — эти роли назначаются пользователям проекта и позволяют им управлять ресурсами в рамках **указанного проекта**.
- [Manage-роли](#manage-роли) — эти роли предназначены для администраторов платформы, предоставляя им права на управление ресурсами на уровне всей платформы.

Права доступа на платформе настраиваются с использованием стандартного подхода RBAC Kubernetes, что предполагает создание ресурсов `RoleBinding` или `ClusterRoleBinding`, в которых указывается соответствующая роль.

### Use-роли

Use-роли предназначены для назначения прав пользователю **в указанном проекте** и определяют доступ к проектным ресурсам. Эти роли могут быть использованы только в контексте ресурса `RoleBinding`.

Платформа предоставляет следующие use-роли:

- `d8:use:role:viewer` — дает возможность просматривать проектные ресурсы и аутентифицироваться в кластере;
- `d8:use:role:user` — дополнительно к роли `d8:use:role:viewer` позволяет просматривать секреты и ресурсы RBAC, подключаться к виртуальным машинам и выполнять команду `d8 k proxy`;
- `d8:use:role:manager` — дополнительно к роли `d8:use:role:user` позволяет управлять проектными ресурсами;
- `d8:use:role:admin` — дополнительно к роли `d8:use:role:manager` позволяет управлять ресурсами `ResourceQuota`, `ServiceAccount`, `Role`, `RoleBinding`, `NetworkPolicy`, `VirtualImage`.

Пример назначения прав администратора проекта `vms` пользователю `joe`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: project-admin-joe
  namespace: vms
subjects:
- kind: User
  name: joe@example.com # для users.deckhouse.io параметр .spec.email
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:use:role:admin
  apiGroup: rbac.authorization.k8s.io
```

Пример назначения прав администратора проекта `vms` группе пользователей `vms-admins`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: project-admin-joe
  namespace: vms
subjects:
- kind: Group
  name: vms-admins # для groups.deckhouse.io параметр .spec.name
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:use:role:admin
  apiGroup: rbac.authorization.k8s.io
```

### Manage-роли

Manage-роли предназначены для предоставления прав на управление:

- Кластерными ресурсами платформы.
- Настройками модулей платформы.
- Компонентами модулей в проектах с префиксами `d8-*`, `kube-*`.

Платформа предоставляет следующие manage-роли, позволяющие управлять всеми подсистемами кластера `all`:

- `d8:manage:all:viewer` — предоставляет права на просмотр конфигураций модулей (ресурсы `moduleConfig`), а также на доступ к cluster-wide-ресурсам этих модулей;
- `d8:manage:all:manager` — предоставляет все права роли `viewer`, а также возможность управления конфигурацией модулей и cluster-wide-ресурсами этих модулей.

Пример назначения прав администратора кластера пользователю `joe`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cluster-admin-joe
subjects:
- kind: User
  name: joe@example.com # для users.deckhouse.io параметр .spec.email
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:manage:all:admin # название manage-роли
  apiGroup: rbac.authorization.k8s.io
```

В платформе предусмотрена возможность предоставления администраторам ограниченных прав для управления ресурсами и модулями, связанными с конкретными подсистемами.

Чтобы назначить права администратора сетевой подсистемы пользователю `joe`, можно использовать следующую настройку:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: network-admin-joe
subjects:
- kind: User
  name: joe@example.com # для users.deckhouse.io параметр .spec.email
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:manage:networking:admin # название manage-роли
  apiGroup: rbac.authorization.k8s.io
```

Роли для управления подсистемами следуют определенному формату — `d8:manage:<SUBSYSTEM>:<ACCESS_LEVEL>`, где:

- `SUBSYSTEM` — название подсистемы;
- `ACCESS_LEVEL` — уровень доступа, по аналогии с ролями подсистемы `all`.

Подсистемы для manage-ролей представлены в таблице:

{% include rbac/rbac-subsystems-list.liquid %}
