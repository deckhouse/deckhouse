---
title: "Модуль user-authz: Custom Resources"
---

## ClusterAuthorizationRule

* `subjects` — Пользователи и/или группы, которым вы хотите предоставить права. [Спецификация](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#subject-v1-rbac-authorization-k8s-io).
    * **Важно!** При использовании совместно с модулем [user-authn](/modules/150-user-authn/), для выдачи прав конкретному пользователю в качестве имени необходимо указывать его `email`.
* `accessLevel` — `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterAdmin`, `SuperAdmin`. Не обязательный параметр.
    * `User` — позволяет получать информацию обо всех объектах (включая доступ к журналам подов), но не позволяет заходить в контейнеры, читать секреты и выполнять port-forward;
    * `PrivilegedUser` — то же самое, что и User, но позволяет заходить в контейнеры, читать секреты, а также позволяет удалять поды (что обеспечивает возможность перезагрузки);
    * `Editor` — то же самое, что и PrivilegedUser, но предоставляет возможность создавать, изменять и удалять namespace и все объекты, которые обычно нужны для прикладных задач;
      * **Важно!** т.к. Editor уполномочен редактировать RoleBindings, он может сам себе расширить полномочия в рамках namespace.
    * `Admin` — то же самое, что и Editor, но позволяет удалять служебные объекты (производные ресурсы, например, ReplicaSet'ы, certmanager.k8s.io/challenges и certmanager.k8s.io/orders);
    * `ClusterEditor` — то же самое, что и Editor, но позволяет управлять ограниченным набором cluster-wide объектов, которые могут понадобиться для прикладных задач (ClusterXXXMetric, ClusterRoleBindings, KeepalivedInstance, DaemonSet...). Роль для работы оператора кластера.
      * **Важно!** т.к. ClusterEditor уполномочен редактировать ClusterRoleBindings, он может сам себе расширить полномочия.
    * `ClusterAdmin` — то же самое, что и ClusterEditor + Admin, но позволяет управлять служебными cluster-wide объектами (производные ресурсы, например, MachineSets, Machines, OpenstackInstanceClasses..., а так же ClusterAuthorizationRule). Роль для работы администратора кластера.
      * **Важно!** т.к. ClusterAdmin уполномочен редактировать ClusterRoleBindings, он может сам себе расширить полномочия.
    * `SuperAdmin` — разрешены любые действия с любыми объектами, при этом ограничения `limitNamespaces` (см. ниже) продолжат работать.
* `portForwarding` — возможные значения `true`, `false` разрешить выполнять `port-forward`;
    * По умолчанию `false`.
* `allowScale` — возможные значения `true`, `false` разрешить масштабировать (выполнять scale) Deployment'ы и StatefulSet'ы;
    * По умолчанию `false`.
* `limitNamespaces` — белый список разрешённых namespace в формате регулярных выражений.
    * Политика:
        * Если список указан, то разрешаем доступ только по нему.
        * Если список не указан, то считаем, что разрешено всё, кроме системных namespace (см. `spec.allowAccessToSystemNamespaces` ниже).
    * Опция доступна только с включённым параметром `enableMultiTenancy`.
* `allowAccessToSystemNamespaces` — разрешить пользователю доступ в служебные namespace (`["antiopa", "kube-.*", "d8-.*", "loghouse", "default"]`).
    * По умолчанию доступа в служебные namespace у пользователей нет.
    * Опция доступна только с включённым параметром `enableMultiTenancy`.
* `additionalRoles` — какие дополнительные роли необходимо выдать для заданных `subjects`.
    * Параметр сделан на крайний случай, вместо него категорически рекомендуется использовать `accessLevel`.
    * Фомат:
    ```yaml
    additionalRoles:
    - apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: cluster-write-all
    - apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: cluster-read-all
    ```
