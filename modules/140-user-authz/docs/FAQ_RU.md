---
title: "Модуль user-authz: FAQ"
---

## Как создать пользователя?

[Создание пользователя](usage.html#создание-пользователя).

## Как ограничить права пользователю конкретными namespace?

Использовать параметр `limitNamespaces` в CR [`ClusterAuthorizationRule`](../../modules/140-user-authz/cr.html#clusterauthorizationrule).

## Что если два ClusterAuthorizationRules подходят для одного пользователя?

Представьте что пользователь `jane.doe@example.com` состоит в группе `administrators`. Созданы два cluster authorization rules:

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
  limitNamespaces:
  - review-.*
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
  limitNamespaces:
  - prod
  - stage
```

1. `jane.doe@example.com` имет права запрашивать и просматривать объекты среди всех review namespace'ов.
2. `Administrators` могут запрашивать, редактировать, получать и удалять объекты на уровне кластера и из namespace'ов `prod` и `stage`.

Так как для `Jane Doe` подходят два правила, необходимо провести вычисления:
* Она будет иметь самый сильный accessLevel среди всех подходящих правил — `ClusterAdmin`.
* Опции `limitNamespaces` будут объединены так, что Jane будет иметь доступ в их общее множество.

Итоговые выданные права будут такими:

```yaml
accessLevel: ClusterAdmin
limitNamespaces:
- prod
- stage
- review-.*
```

> **Note!** Если есть правило без опции limitNamespaces, это значит, что доступ разрешен во все namespace'ы, кроме системных, что повлияет на результат вычисления доступных namespace для пользователя.
