---
title: "Модуль user-authz" 
---

Данный модуль отвечает за генерацию RBAC для пользователей и реализует простейший multi-tenancy с разграничением доступа по namespace.

Реализует role-based подсистему сквозной авторизации, расширяя функционал стандартного механизма RBAC.

Вся настройка прав доступа происходит с помощью [Custom Resources](cr.html).

## Возможности модуля

- Управление доступом пользователей и групп на базе механизма RBAC Kubernetes
- Управление доступом к инструментам масштабирования (параметр `allowScale` Custom Resource [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule))
- Управление доступом к форвардингу портов (параметр `portForwarding` Custom Resource [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule))
- Управление списком разрешенных namespace в формате регулярных выражений (параметр `limitNamespaces` Custom Resource [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule))
- Управление доступом к системным namespace (параметр `allowAccessToSystemNamespaces` Custom Resource [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule)), таким как `kube-system` и пр;

## Ролевая модель
В модуле кроме прямого использования RBAC можно использовать удобный набор высокоуровневых ролей:
- `User` — позволяет получать информацию обо всех объектах (включая доступ к журналам подов), но не позволяет заходить в контейнеры, читать секреты и выполнять port-forward;
- `PrivilegedUser` — то же самое, что и User, но позволяет заходить в контейнеры, читать секреты, а также позволяет удалять поды (что обеспечивает возможность перезагрузки);
- `Editor` — то же самое, что и `PrivilegedUser`, но предоставляет возможность создавать и изменять namespace и все объекты, которые обычно нужны для прикладных задач. **Обратите внимание**, что так как `Editor` уполномочен редактировать `RoleBindings`, он может **сам себе расширить полномочия в рамках namespace**;
- `Admin` — то же самое, что и `Editor`, но позволяет удалять служебные объекты (производные ресурсы, такие как `ReplicaSet`, `certmanager.k8s.io/challenges` и `certmanager.k8s.io/orders`);
- `ClusterEditor` — то же самое, что и `Editor`, но позволяет управлять ограниченным набором `cluster-wide`-объектов, которые могут понадобиться для прикладных задач, таких как `ClusterXXXMetric`, `ClusterRoleBindings`, `KeepalivedInstance`, `DaemonSet` и т.п.. Роль для работы оператора кластера. **Важно**, что так как `ClusterEditor` уполномочен редактировать `ClusterRoleBindings`, он может **сам себе расширить полномочия**.
- `ClusterAdmin` — то же самое, что и `ClusterEditor` и `Admin` вместе взятые, кроме того данная роль позволяет управлять служебными cluster-wide объектами (производные ресурсы, например, `MachineSets`, `Machines`, `OpenstackInstanceClasses` и т.п.). Роль для работы администратора кластера. **Важно**, что так как `ClusterAdmin` уполномочен редактировать `ClusterRoleBindings`, он может **сам себе расширить полномочия**.
- `SuperAdmin` — разрешены любые действия с любыми объектами, при этом ограничения [`limitNamespaces`](#возможности-модуля) продолжат работать.

## Особенности реализации
**Важно!** Режим multi-tenancy (авторизация по namespace) в данный момент реализован по временной схеме и **не гарантирует безопасность!**

Если webhook, который реализовывает систему авторизации по какой-то причине будет недоступен, то в это время опции `allowAccessToSystemNamespaces` и `limitNamespaces` в CR перестанут применяться и пользователи будут иметь доступ во все namespace. После восстановления доступности webhook'а опции продолжат работать.

