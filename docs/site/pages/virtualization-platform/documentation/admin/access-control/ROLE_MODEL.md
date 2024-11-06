---
title: "Role model"
permalink: en/virtualization-platform/documentation/admin/access-control/role-model.html
lang: en
---

## Описание

Платформа предоставляет стандартный набор ролей для доступа к проектным и кластерным ресурсам, которые делятся на два класса:
- [Use-роли](#use-роли) — для назначения прав пользователям проекта для управления ресурсами **в указанном проекте**.
- [Manage-роли](#manage-роли) — для назначения прав администраторам платформы.

Настройка прав доступа к ресурсам платформы выполняется стандартным для RBAC Kubernetes способом: с помощью создания ресурсов `RoleBinding` или `ClusterRoleBinding`, с указанием в них одной из ролей.

### Use-роли

Use-роли предназначены для назначения прав пользователю **в указанном проекте** и определяет права на доступ к проектным ресурсам, использовать данную роль можно только в ресурсе `RoleBinding`

Платформа предоставляет для использования следующие use-роли:
- `d8:use:role:viewer` — позволяет просматривать проектные  ресурсы, а также выполнять аутентификацию в кластере;
- `d8:use:role:user` — дополнительно к роли `d8:use:role:viewer` позволяет просматривать секреты и ресурсы RBAC, подключаться к виртуальными машинам, выполнять `d8 k proxy`;
- `d8:use:role:manager` — дополнительно к роли `d8:use:role:user` позволяет управлять проектными-ресурсами;
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
  name: joe
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

TODO:
Платформа предоставляет следующие manage-роли:
- `d8:manage:all:viewer` — позволяет просматривать конфигурацию модулей (ресурсы `moduleConfig`), cluster-wide-ресурсы модулей;
- `d8:manage:all:user` — дополнительно к роли `viewer` ?????????????????????????????????????????????;
- `d8:manage:all:manager` — дополнительно к роли `user` позволяет управлять конфигурацией модулей (ресурсы `moduleConfig`), cluster-wide-ресурсами модулей;
- `d8:manage:all:admin` — дополнительно к роли `manager` позволяет управлять такими ресурсами, как `CustomResourceDefinition`, `Namespace`, `Node`, `ClusterRole`, `ClusterRoleBinding`, `PersistentVolume`, `MutatingWebhookConfiguration`, `ValidatingAdmissionPolicy` и т. п.

Пример назначения прав администратора кластера пользователю `joe`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cluster-admin-joe
subjects:
- kind: User
  name: joe # пользователь
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:manage:all:admin # название manage-роли
  apiGroup: rbac.authorization.k8s.io
```

Существует возможность предоставления администраторам платформы детализированных прав для управления ресурсами и модулями в определённой области.

В качестве примера, рассмотрим назначение прав администратора сетевой подсистемы для пользователя `joe`.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: network-admin-joe
subjects:
- kind: User
  name: joe # пользователь
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:manage:networking:admin # название manage-роли
  apiGroup: rbac.authorization.k8s.io
```

Формат названия таких ролей выглядит следующим образом `d8:manage:<SCOPE>:<ACCESS_LEVEL>`, где:
- `SCOPE` — область роли.
- `ACCESS_LEVEL` — уровень доступа, по аналогии с ролями области `all`.

Области для manage-ролей представлены в таблице:

TODO: ТУТ ТАБЛИЦА
{/* include rbac-scopes-list.liquid */}
