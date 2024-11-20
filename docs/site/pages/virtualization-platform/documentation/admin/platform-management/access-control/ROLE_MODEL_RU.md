---
title: "Ролевая модель"
permalink: ru/virtualization-platform/documentation/admin/platform-management/access-control/role-model.html
lang: ru
---

## Описание

Платформа предоставляет стандартный набор ролей для доступа к проектным и кластерным ресурсам, которые делятся на два класса:
- [Use-роли](#use-роли) — для назначения прав пользователям проекта для управления ресурсами **в указанном проекте**.
- [Manage-роли](#manage-роли) — для назначения прав администраторам платформы.

Настройка прав доступа к ресурсам платформы выполняется стандартным для RBAC Kubernetes способом: с помощью создания ресурсов RoleBinding или ClusterRoleBinding, с указанием в них одной из ролей.

### Use-роли

Use-роли предназначены для назначения прав пользователю **в указанном проекте** и определяют права на доступ к проектным ресурсам, использовать данные роли можно только в ресурсе RoleBinding

Платформа предоставляет для использования следующие use-роли:
- `d8:use:role:viewer` — позволяет просматривать проектные  ресурсы, а также выполнять аутентификацию в кластере;
- `d8:use:role:user` — дополнительно к роли `d8:use:role:viewer` позволяет просматривать секреты и ресурсы RBAC, подключаться к виртуальными машинам, выполнять `d8 k proxy`;
- `d8:use:role:manager` — дополнительно к роли `d8:use:role:user` позволяет управлять проектными-ресурсами;
- `d8:use:role:admin` — дополнительно к роли `d8:use:role:manager` позволяет управлять ресурсами ResourceQuota, ServiceAccount, Role, RoleBinding, NetworkPolicy, VirtualImage.

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
- `d8:manage:all:viewer` — позволяет просматривать конфигурацию модулей (ресурсы `moduleConfig`), cluster-wide-ресурсы модулей;
- `d8:manage:all:manager` — дополнительно к роли `viewer` позволяет управлять конфигурацией модулей (ресурсы `moduleConfig`), cluster-wide-ресурсами модулей;

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

Существует возможность предоставления администраторам платформы ограниченных прав для управления ресурсами и модулями относящимся к определенным подсистемам.

В качестве примера, рассмотрим назначение прав администратора сетевой подсистемы для пользователя `joe`.

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

Формат названия таких ролей выглядит следующим образом `d8:manage:<SUBSYSTEM>:<ACCESS_LEVEL>`, где:
- `SUBSYSTEM` — название подсистемы.
- `ACCESS_LEVEL` — уровень доступа, по аналогии с ролями подсистемы `all`.

Подсистемы для manage-ролей представлены в таблице:

TODO: ТУТ ТАБЛИЦА
{/* include rbac-scopes-list.liquid */}
