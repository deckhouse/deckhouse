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

## Как расширить роли или создать новую ?

Новая ролевая модель построена на принципе агрегации, она собирает более мелкие роли в более обширные, 
поэтому предоставляет легкий способы расширения модели собственными ролями.

1. Создание новой роли области.

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

    Разбирая по порядку, первым делом нам нужно указать лейблы для нашей роли:
    - Этот лейбл нужно указывать обязательно, чтобы показать что роль должна обрабатываться как manage роль.
    ```yaml
    rbac.deckhouse.io/kind: manage
    ```
    
    - Этот лейбл показывает, что роль является ролью области, и обрабатываться будет соответственно.
    ```yaml
    rbac.deckhouse.io/level: scope
    ```
   
    - Этот лейбл нужен для указания области за которую отвечает роль.
    ```yaml
    rbac.deckhouse.io/scope: custom
    ```
   
    - Этот лейбл позволяет manage:all роли сагрегировать эту роль.
    ```yaml
    rbac.deckhouse.io/aggregate-to-all-as: admin
    ```
   Следующая часть это селектор, и именно они реализуют агрегацию:
   - Этот селектор агрегирует роль админа из области deckhouse.
    ```yaml
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/aggregate-to-deckhouse-as: admin
    ```
   - Данный селектор сагрерирует все правила от модуля user-authn
   ```yaml
    rbac.deckhouse.io/kind: manage
    module: user-authn
   ```

   Таким образом наша роль получает права от области deckhouse, kubernetes и от модуля user-authn.

   * Имена роли области должны придерживаться такой стилистики, потому что последнее слово после ':' определяет какая use роль будет создана в неймспейсах.
   * Ограничений на имена капабилити нет, но для читаемости лучше использовать этот стиль.
   * Use роли будут созданы в агрегированных областях и неймспейсе модуля.

2. Расширение пользовательской роли.

   Допустим в кластере появился новый кластерный(пример для manage роли)CRD объект - MySuperResource, и нам нужно дополнить
   нашу собственную роль(пример выше) правами на взаимодействие с этим ресурсом.

    Первым делом нам нужно дополнить нашу роль новым селектором:
    ```yaml
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/aggregate-to-custom-as: admin
    ```
   Этот селектор позволит нам агрегировать роли(капабилити) к новой области через указание этого лейбла.
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

   * Имена роли области должны придерживаться такой стилистики, потому что последнее слово после ':' определяет какая use роль будет создана в неймспейсах.
   * Ограничений на имена капабилити нет, но для читаемости лучше использовать этот стиль.

3. Расширение существующих manage scope ролей.

   Если мы хотим расширить существующую роль, мы можем использовать тот же путь выше, но изменив лейблы и название роли.
   Пример для расширения роли админа из области deckhouse(```d8:manage:deckhouse:admin```):
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

   Таким образом эта роль расширит роль```d8:manage:deckhouse```.

4. Расширение существующих manage scope ролей с добавлением нового неймспейса.

   Если мы хотим добавить новый неймспейс(для создания use роли там с помощью хука), нам потребуется добавить лишь один лейбл:
   ```yaml
   "rbac.deckhouse.io/namespace": namespace
   ```
   Этот лейбл сообщает хуку, что в этом неймспейсе нужно создать use роль.
    ```yaml
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      labels:
        rbac.deckhouse.io/aggregate-to-deckhouse-as: admin
        rbac.deckhouse.io/kind: manage
        rbac.deckhouse.io/namespace: namespace
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

   Как это работает ? Хук мониторит ClusterRoleBinding, и при создании биндинга, он ходит по всем manage ролям, 
   чтобы найти все сагрерированные роли с помощью проверки правила агрегации, затем он берет неймспейс из лейбла ```rbac.deckhouse.io/namespace```,
   и создает use роль в этом неймспейсе, use роль определяется последним словом после ':' в областной роли(в нашем случае - admin).

5. Расширение существующих use ролей.

   Если наш ресурс неймспейсный, нам нужно расширить use роль вместо manage роли. Разница лишь в лейблах и имени:
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

   И эта роль дополнит роль ```d8:use:role:user```.
