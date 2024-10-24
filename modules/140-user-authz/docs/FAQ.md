---
title: "The user-authz module: FAQ"
---

## How do I create a user?

[Creating a user](usage.html#creating-a-user).

## How do I limit user rights to specific namespaces?

To limit a user's rights to specific namespaces, use `RoleBinding` with the [use role](./#use-roles) that has the appropriate level of access. [Example...](usage.html#example-of-assigning-administrative-rights-to-a-user-within-a-namespace).

### How do I limit user rights to specific namespaces (obsolete role-based model)?

{% alert level="warning" %}
The example uses the [obsolete role-based model](./#the-obsolete-role-based-model).
{% endalert %}

Use the `namespaceSelector` or `limitNamespaces` (deprecated) parameters in the [`ClusterAuthorizationRule`](../../modules/140-user-authz/cr.html#clusterauthorizationrule) CR.

## What if there are two ClusterAuthorizationRules matching to a single user?

Imagine that the user `jane.doe@example.com` is in the `administrators` group. There are two cluster authorization rules:

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

1. `jane.doe@example.com` has the right to get and list any objects in the namespaces labeled `env=review`
2. `Administrators` can get, edit, list, and delete objects on the cluster level and in the namespaces labeled `env=prod` and `env=stage`.

Because `Jane Doe` matches two rules, some calculations will be made:
* She will have the most powerful accessLevel across all matching rules — `ClusterAdmin`.
* The `namespaceSelector` options will be combined, so that Jane will have access to all the namespaces labeled with `env` label of the following values: `review`, `stage`, or `prod`.

> **Note!** If there is a rule without the `namespaceSelector` option and `limitNamespaces` deprecated option, it means that all namespaces are allowed excluding system namespaces, which will affect the resulting limit namespaces calculation.

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
