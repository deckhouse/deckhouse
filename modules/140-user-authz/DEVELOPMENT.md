Для разработчиков
=================

## Настройка дополнительных ClusterRole для разных accessLevel

Если требуется добавить прав для определённого accessLevel, то достаточно создать ClusterRole с аннотацией `user-authz.deckhouse.io/access-level: <AccessLevel>`. Пример:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  annotations:
    user-authz.deckhouse.io/access-level: Master
  name: mymodule:master
rules:
- apiGroups:
  - mymodule.io
  resources:
  - destinationrules
  - virtualservices
  - serviceentries
  verbs:
  - create
  - list
  - get
  - update
  - delete
```
