---
title: "Модуль user-authz: FAQ"
---

## Как создать пользователя?

[Создание пользователя](usage.html#создание-пользователя).

## Как ограничить права пользователю конкретными namespace?

Использовать параметры `namespaceSelector` или `limitNamespaces` (устарел) в custom resource [`ClusterAuthorizationRule`](../../modules/140-user-authz/cr.html#clusterauthorizationrule).

## Что, если два ClusterAuthorizationRules подходят для одного пользователя?

Представьте, что пользователь `jane.doe@example.com` состоит в группе `administrators`. Созданы два ClusterAuthorizationRules:

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: jane
spec:
  subjects:
    - kind: User
      name: jane.doe@example.com
  accessLevel: User
  namespaceSelector:
    labelSelector:
      matchLabels:
        env: review
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  subjects:
  - kind: Group
    name: administrators
  accessLevel: ClusterAdmin
  namespaceSelector:
    labelSelector:
      matchExpressions:
      - key: env
        operator: In
        values:
        - prod
        - stage
```

1. `jane.doe@example.com` имеет право запрашивать и просматривать объекты среди всех namespace'ов, помеченных `env=review`.
2. `Administrators` могут запрашивать, редактировать, получать и удалять объекты на уровне кластера и из namespace'ов, помеченных `env=prod` и `env=stage`.

Так как для `Jane Doe` подходят два правила, необходимо провести вычисления:
* Она будет иметь самый сильный accessLevel среди всех подходящих правил — `ClusterAdmin`.
* Опции `namespaceSelector` будут объединены так, что `Jane Doe` будет иметь доступ в namespace'ы, помеченные меткой `env` со значением `review`, `stage` или `prod`.

> **Note!** Если есть правило без опции `namespaceSelector` и без опции `limitNamespaces` (устаревшая), это значит, что доступ разрешен во все namespace'ы, кроме системных, что повлияет на результат вычисления доступных namespace'ов для пользователя.
