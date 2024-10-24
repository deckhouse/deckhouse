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

## Как расширить роли или создать новую?

Новая ролевая модель построена на принципе агрегации, она собирает более мелкие роли в более обширные,
тем самым предоставляя легкие способы расширения модели собственными ролями.

### Создание новой роли области

Предположим, что текущие области не подходят под ролевое распределение в компании и требуется создать новую область,
которая будет включать в себя роли из области `deckhouse`, области `kubernetes` и модуля user-authn.

Для решения этой задачи создайте следующую роль:

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

Вначале указаны лейблы для новой роли:

- показывает, что роль должна обрабатываться как manage роль:

  ```yaml
  rbac.deckhouse.io/kind: manage
  ```

  > Этот лейбл должен быть обязательно указан!

- показывает, что роль является ролью области, и обрабатываться будет соответственно:

  ```yaml
  rbac.deckhouse.io/level: scope
  ```

- указывает область, за которую отвечает роль:

  ```yaml
  rbac.deckhouse.io/scope: custom
  ```

- позволяет `manage:all` роли сагрегировать эту роль:

  ```yaml
  rbac.deckhouse.io/aggregate-to-all-as: admin
  ```

Далее указаны селекторы, именно они реализуют агрегацию:

- агрегирует роль админа из области `deckhouse`:

  ```yaml
  rbac.deckhouse.io/kind: manage
  rbac.deckhouse.io/aggregate-to-deckhouse-as: admin
  ```

- агрерирует все правила от модуля user-authn:

  ```yaml
   rbac.deckhouse.io/kind: manage
   module: user-authn
  ```

Таким образом роль получает права от областей `deckhouse`, `kubernetes` и от модуля user-authn.

Особенности:

* имена роли области должны придерживаться стилистики `custom:manage:custom:admin`, т.к. последнее слово после `:` определяет, какая use-роль будет создана в пространствах имён;
* ограничений на имена капабилити нет, но для читаемости лучше использовать этот стиль;
* use-роли будут созданы в агрегированных областях и пространстве имён модуля.

### Расширение пользовательской роли

Например, в кластере появился новый кластерный (пример для manage роли) CRD-объект - MySuperResource, и нужно дополнить собственную роль из примера выше правами на взаимодействие с этим ресурсом.

Первым делом нужно дополнить роль новым селектором:

```yaml
rbac.deckhouse.io/kind: manage
rbac.deckhouse.io/aggregate-to-custom-as: admin
```

Этот селектор позволит агрегировать роли (капабилити) к новой области через указание этого лейбла. После добавление нового селектора роль будет выглядеть так:

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

 Далее нужно создать новую роль (капабилити), в которой определить права для нового ресурса. Например, только чтение:

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

Роль дополнит своими правами роль области, дав права на просмотр нового объекта.

Особенности:

* имена роли области должны придерживаться стилистики `custom:manage:custom:admin`, т.к. последнее слово после `:` определяет, какая use-роль будет создана в пространствах имён;
* ограничений на имена капабилити нет, но для читаемости лучше использовать этот стиль.

### Расширение существующих manage scope ролей

Если необходимо расширить существующую роль, нужно выполнить те же шаги, что и в пункте выше, но изменив лейблы и название роли.

Пример для расширения роли админа из области `deckhouse`(`d8:manage:deckhouse:admin`):

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

Таким образом новая роль расширит роль `d8:manage:deckhouse`.

### Расширение существующих manage scope ролей с добавлением нового пространства имён

Если необходимо добавить новое пространство имён (например, для создания в нём use-роли с помощью хука), потребуется добавить лишь один лейбл:

```yaml
"rbac.deckhouse.io/namespace": namespace
```

Этот лейбл сообщает хуку, что в этом пространстве имён нужно создать use-роль:

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

Хук мониторит `ClusterRoleBinding`, и при создании биндинга он ходит по всем manage-ролям, чтобы найти все сагрерированные роли с помощью проверки правила агрегации. Затем он берет пространство имён из лейбла `rbac.deckhouse.io/namespace` и создает use-роль в этом пространстве имён. Use-роль определяется последним словом после `:` в областной роли (в примере выше - `admin`).

### Расширение существующих use-ролей

Если ресурс принадлежит пространству имён, необходимо расширить use-роль вместо manage-роли. Разница лишь в лейблах и имени:

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

Эта роль дополнит роль `d8:use:role:user`.
