---
title: "Модуль user-authz: FAQ"
---

## Как создать пользователя?

[Создание пользователя](usage.html#создание-пользователя).

## Как ограничить права пользователю конкретными пространствами имён?

Чтобы ограничить права пользователя конкретными пространствами имён, используйте в `RoleBinding` [use-роль](./#use-роли) с соответствующим уровнем доступа. [Пример...](usage.html#пример-назначения-административных-прав-пользователю-в-рамках-пространства-имён).

### Как ограничить права пользователю конкретными пространствами имён (устаревшая ролевая модель)

{% alert level="warning" %}
Используется [устаревшая ролевая модель](./#устаревшая-ролевая-модель).
{% endalert %}

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

1. `jane.doe@example.com` имеет право запрашивать и просматривать объекты среди всех пространств имён, помеченных `env=review`.
2. `Administrators` могут запрашивать, редактировать, получать и удалять объекты на уровне кластера и из пространств имён, помеченных `env=prod` и `env=stage`.

Так как для `Jane Doe` подходят два правила, необходимо провести вычисления:
* Она будет иметь самый сильный accessLevel среди всех подходящих правил — `ClusterAdmin`.
* Опции `namespaceSelector` будут объединены так, что `Jane Doe` будет иметь доступ в пространства имён, помеченные меткой `env` со значением `review`, `stage` или `prod`.

> **Note!** Если есть правило без опции `namespaceSelector` и без опции `limitNamespaces` (устаревшая), это значит, что доступ разрешен во все пространства имён, кроме системных, что повлияет на результат вычисления доступных пространств имён для пользователя.

## Как создать собственные роль или расширять текущие ?

Новая ролевая модель построена на принципе агрегации более мелких ролей в более обширные, 
поэтому предоставляет способы расширения модели собственными ролями.

1. Создание собственной роли области.

    Предположим текущие области не подходят под ролевое распределение в компании и требуется создать новую область, 
    которая будет включать в себя роли из области deckhouse, области kubernetes и модуля user-authn.
    
    Для решения этой задачи мы можем создать следующую роль:
    
    ```yaml
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      name: custom:manage:custom:admin
      labels:
        rbac.deckhouse.io/kind: manage
        rbac.deckhouse.io/level: scope
        rbac.deckhouse.io/scope: custom
        rbac.deckhouse.io/aggregate-to-all-as: admin
    aggregationRule:
      clusterRoleSelectors:
        - matchLabels:
            rbac.deckhouse.io/kind: manage
            rbac.deckhouse.io/aggregate-to-deckhouse-as: admin
        - matchLabels:
            rbac.deckhouse.io/kind: manage
            rbac.deckhouse.io/aggregate-to-kubernetes-as: admin
        - matchLabels:
            rbac.deckhouse.io/kind: manage
            module: user-authn
    rules: []
    ```

    Разбирая по порядку первым делом нам нужно указать лейблы для нашей роли:
    - Этот лейбл нужно указывать обязательно, чтобы показать что роль должна обрабатываться как manage роль.
    ```yaml
    rbac.deckhouse.io/kind: manage
    ```
    
    - Этот лейбл показывает, что роль является областной ролью, и обрабатываться будет соответственно.
    ```yaml
    rbac.deckhouse.io/level: scope
    ```
   
    - Этот лейбл нужен для указания области за которую отвечает роль.
    ```yaml
    rbac.deckhouse.io/scope: custom
    ```
   
    - Этот лейбл позволяет all роли сагрегировать эту роль.
    ```yaml
    rbac.deckhouse.io/aggregate-to-all-as: admin
    ```
   
    Далее идет уже само указание того, что должно быть в этой роли:
    - Данный селектор сагрерирует все правила для админа от области deckhouse.
    ```yaml
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/aggregate-to-deckhouse-as: admin
    ```
   - Данный селектор сагрерирует все правила от модуля user-authn
   ```yaml
    rbac.deckhouse.io/kind: manage
    module: user-authn
   ```
   
    Таким образом наша роль получит права в области deckhouse, kubernetes и модуле user-authn.
    * Use роли будут созданы в неймспейсах области и модуля.
    * Привязки к названиям нет, но лучше сохранять стилистику для читаемости.

2. Расширение собственной роли.

    Допустим в кластере появился новый кластерный(пример для manage роли)CRD объект - MySuperResource, и нам нужно дополнить 
    нашу собственную роль(пример выше) правами на взаимодействие с этим ресурсом.
    
    Первым делом нам нужно дополнить нашу роль новым селектором:
    ```yaml
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/aggregate-to-custom-as: admin
    ```
   Этот селектор позволит нам агрегировать роли(капабилити) к это новой области через указание этого лейбла.
   После добавление нового селектора, роль будет выглядеть так:
    ```yaml
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      name: custom:manage:custom:admin
      labels:
        rbac.deckhouse.io/kind: manage
        rbac.deckhouse.io/level: scope
        rbac.deckhouse.io/scope: custom
        rbac.deckhouse.io/aggregate-to-all-as: admin
    aggregationRule:
      clusterRoleSelectors:
        - matchLabels:
            rbac.deckhouse.io/kind: manage
            rbac.deckhouse.io/aggregate-to-deckhouse-as: admin
        - matchLabels:
            rbac.deckhouse.io/kind: manage
            rbac.deckhouse.io/aggregate-to-kubernetes-as: admin
        - matchLabels:
            rbac.deckhouse.io/kind: manage
            module: user-authn
        - matchLabels:
            rbac.deckhouse.io/kind: manage
            rbac.deckhouse.io/aggregate-to-custom-as: admin
    rules: []
    ```
   
    Далее нужно создать нашу новую роль(капабилити) в которой определить права для нашего ресурса, для примера сделаем только чтение:
    ```yaml
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      labels:
        rbac.deckhouse.io/aggregate-to-custom-as: admin
        rbac.deckhouse.io/kind: manage
      name: custom:manage:capability:custom:superresource:view
    rules:
    - apiGroups:
      - mygroup.io
      resources:
      - mysuperresources
      verbs:
      - get
      - list
      - watch
    ```
   
    Агрегацию определяет этот лейбл, который мы указывали выше в селекторе.
    ```yaml
    rbac.deckhouse.io/aggregate-to-custom-as: admin
    ```
   
    Данная роль дополнит своими правами нашу роль области дав права на просмотр объекта.

    * Привязки к названиям нет, но лучше сохранять стилистику для читаемости.

3. Расширение текущей manage роли.

    Если мы хотим расширить какую-то область, то по аналогии с примером выше нужно будет лишь указать другой лейбл агрегации.
    Пример роли для расширения deckhouse области(d8:manage:deckhouse:admin):
    ```yaml
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      labels:
        rbac.deckhouse.io/aggregate-to-deckhouse-as: admin
        rbac.deckhouse.io/kind: manage
      name: custom:manage:capability:custom:superresource:view
    rules:
    - apiGroups:
      - mygroup.io
      resources:
      - mysuperresources
      verbs:
      - get
      - list
      - watch
    ```
   
   Данная роль расширит d8:manage:deckhouse:admin роль.

4. Расширение текущей use роли.

    Если наш ресурс является неймспейсным, то расширять нужно use роль, а не manage. Разница будет лишь в двух лейблах и названии:
    ```yaml
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      labels:
        rbac.deckhouse.io/aggregate-to-role: user
        rbac.deckhouse.io/kind: use
      name: custom:use:capability:custom:superresource:view
    rules:
    - apiGroups:
      - mygroup.io
      resources:
      - mysuperresources
      verbs:
      - get
      - list
      - watch
    ```
   
    Данная роль расширит d8:use:role:user роль.
