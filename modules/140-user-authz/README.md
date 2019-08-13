Модуль user-authz
=======

Данный модуль отвечает за генерацию RBAC для пользователей.

Конфигурация
------------

Данный модуль не имеет настроек и включен по-умолчанию.

Использование
------------

Вся настройка прав доступа происходит с помощью CRD.

Формат CRD выглядит так:
```yaml
apiVersion: authz.flant.com/v1
kind: ClusterAuthorizationRule
metadata:
  name: test
spec:
  subjects:
  - kind: User
    name: some@example.com
  - kind: Group
    name: some-group-name
  accessLevel: Master
  portForwarding: true
  additionalRoles:
  - apiGroup: rbac.authorization.k8s.io
    kind: ClusterRole
    name: cluster-write-all
  - apiGroup: rbac.authorization.k8s.io
    kind: ClusterRole
    name: cluster-read-all
```

В `spec` возможны такие параметры:
* `subjects` - Пользователи и/или группы, которым вы хотите предоставить права. [Спецификация](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#subject-v1-rbac-authorization-k8s-io).
* `accessLevel` - `User`, `Master`, `Deploy` или `Admin`. Не обязательный параметр.
    * `User` - позволяет получать информацию обо всех объектах (включая доступ к журналам подов), но не позволяет заходить в контейнеры, читать секреты и выполнять port-forward;
    * `Master` - то же самое, что и User, но позволяет заходить в контейнеры, читать секреты, а также позволяет удалять поды (что обеспечивает возможность перезагрузки);
    * `Deploy` - то же самое, что и Master, но предоставляет возможность создавать и изменять namespace и большинство объектов (не позволяет создавать Pod'ы);
    * `Admin` - то же самое, что и Deploy, но позволяет удалять служебные объекты (ReplicaSet'ы, certmanager.k8s.io/challenges и certmanager.k8s.io/orders);
* `portForwarding` - возможные значения `true`, `false` разрешить выполнять `port-forward`;
    * По-умолчанию `false`.
* `allowScale` - возможные значения `true`, `false` разрешить масштабировать (выполнять scale) Deployment'ы и StatefulSet'ы;
    * По-умолчанию `false`.
* `additionalRoles` - какие дополнительные роли необходимо выдать для заданных `subjects`.
