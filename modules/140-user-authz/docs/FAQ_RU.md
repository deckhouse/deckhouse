---
title: "Модуль user-authz: FAQ"
---

## Как создать пользователя?

[Создание пользователя](usage.html#создание-пользователя).

<div style="height: 0;" id="как-ограничить-права-пользователю-конкретными-пространствами-имён-устаревшая-ролевая-модель"></div>

## Как ограничить права пользователю конкретными пространствами имён?

Чтобы ограничить права пользователя конкретными пространствами имён в экспериментальной ролевой модели, используйте в `RoleBinding` [use-роль](./#use-роли) с соответствующим уровнем доступа. [Пример...](usage.html#пример-назначения-административных-прав-пользователю-в-рамках-пространства-имён).

В текущей ролевой модули используйте параметры `namespaceSelector` или `limitNamespaces` (устарел) в кастомном ресурсе [`ClusterAuthorizationRule`](../../modules/user-authz/cr.html#clusterauthorizationrule).

## Что, если два ClusterAuthorizationRules подходят для одного пользователя?

В примере пользователь `jane.doe@example.com` состоит в группе `administrators`. Созданы два ClusterAuthorizationRules:

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

* `Jane Doe` будет иметь самый сильный accessLevel среди всех подходящих правил — `ClusterAdmin`.
* Опции `namespaceSelector` будут объединены так, что `Jane Doe` будет иметь доступ в пространства имён, помеченные меткой `env` со значением `review`, `stage` или `prod`.

{% alert level="warning" %}
Если есть правило без опции `namespaceSelector` и без опции `limitNamespaces` (устаревшая), это значит, что доступ разрешён во все пространства имён, кроме системных, что повлияет на результат вычисления доступных пространств имён для пользователя.
{% endalert %}

## Как расширить роли или создать новую?

[Экспериментальная ролевая модель](./#экспериментальная-ролевая-модель) построена на принципе агрегации, она собирает более мелкие роли в более обширные,
тем самым предоставляя лёгкие способы расширения модели собственными ролями.

### Создание новой роли подсистемы

Предположим, что текущие подсистемы не подходят под ролевое распределение в компании и требуется создать новую [подсистему](./#подсистемы-ролевой-модели),
которая будет включать в себя роли из подсистемы `deckhouse`, подсистемы `kubernetes` и модуля user-authn.

Для решения этой задачи создайте следующую роль:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:manage:mycustom:manager
  labels:
    rbac.deckhouse.io/use-role: admin
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/level: subsystem
    rbac.deckhouse.io/subsystem: custom
    rbac.deckhouse.io/aggregate-to-all-as: manager
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/kind: manage
        rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    - matchLabels:
        rbac.deckhouse.io/kind: manage
        rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
    - matchLabels:
        rbac.deckhouse.io/kind: manage
        module: user-authn
rules: []
```

В начале указаны лейблы для новой роли:

- показывает, какую роль хук должен использовать при создании use ролей:

  ```yaml
  rbac.deckhouse.io/use-role: admin
  ```

- показывает, что роль должна обрабатываться как manage-роль:

  ```yaml
  rbac.deckhouse.io/kind: manage
  ```

  > Этот лейбл обязателен.

- показывает, что роль является ролью подсистемы, и обрабатываться будет соответственно:

  ```yaml
  rbac.deckhouse.io/level: subsystem
  ```

- указывает подсистему, за которую отвечает роль:

  ```yaml
  rbac.deckhouse.io/subsystem: custom
  ```

- позволяет `manage:all`-роли агрегировать эту роль в себя:

  ```yaml
  rbac.deckhouse.io/aggregate-to-all-as: manager
  ```

Далее указаны селекторы, именно они реализуют агрегацию:

- агрегирует роль менеджера из подсистемы `deckhouse`:

  ```yaml
  rbac.deckhouse.io/kind: manage
  rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
  ```

- агрегирует все правила от модуля user-authn:

  ```yaml
   rbac.deckhouse.io/kind: manage
   module: user-authn
  ```

Таким образом роль получает права от подсистем `deckhouse`, `kubernetes` и от модуля user-authn.

Особенности:

* ограничений на имя роли нет, но для читаемости лучше использовать этот стиль;
* use-роли будут созданы в пространстве имён агрегированных подсистем и модуля, тип роли выбран лейблом.

### Расширение пользовательской роли

Например, в кластере появился новый кластерный (пример для manage-роли) CRD-объект — MySuperResource, и нужно дополнить собственную роль из примера выше правами на взаимодействие с этим ресурсом.

Первым делом нужно дополнить роль новым селектором:

```yaml
rbac.deckhouse.io/kind: manage
rbac.deckhouse.io/aggregate-to-custom-as: manager
```

Этот селектор позволит агрегировать роли к новой подсистеме через указание этого лейбла. После добавления нового селектора роль будет выглядеть так:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   name: custom:manage:mycustom:manager
   labels:
     rbac.deckhouse.io/use-role: admin
     rbac.deckhouse.io/kind: manage
     rbac.deckhouse.io/level: subsystem
     rbac.deckhouse.io/subsystem: custom
     rbac.deckhouse.io/aggregate-to-all-as: manager
 aggregationRule:
   clusterRoleSelectors:
     - matchLabels:
         rbac.deckhouse.io/kind: manage
         rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
     - matchLabels:
         rbac.deckhouse.io/kind: manage
         rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
     - matchLabels:
         rbac.deckhouse.io/kind: manage
         module: user-authn
     - matchLabels:
         rbac.deckhouse.io/kind: manage
         rbac.deckhouse.io/aggregate-to-custom-as: manager
 rules: []
 ```

 Далее нужно создать новую роль, в которой следует определить права для нового ресурса. Например, только чтение:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-custom-as: manager
     rbac.deckhouse.io/kind: manage
   name: custom:manage:permission:mycustom:superresource:view
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

Роль дополнит своими правами роль подсистемы, дав права на просмотр нового объекта.

Особенности:

* ограничений на имя роли нет, но для читаемости лучше использовать этот стиль.

### Расширение существующих manage subsystem-ролей

Если необходимо расширить существующую роль, нужно выполнить те же шаги, что и в пункте выше, но изменив лейблы и название роли.

Пример для расширения роли менеджера из подсистемы `deckhouse`(`d8:manage:deckhouse:manager`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    rbac.deckhouse.io/kind: manage
  name: custom:manage:permission:mycustommodule:superresource:view
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

### Расширение manage subsystem-ролей с добавлением нового пространства имён

Если необходимо добавить новое пространство имён (для создания в нём use-роли с помощью хука), потребуется добавить лишь один лейбл:

```yaml
"rbac.deckhouse.io/namespace": namespace
```

Этот лейбл сообщает хуку, что в этом пространстве имён нужно создать use-роль:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
     rbac.deckhouse.io/kind: manage
     rbac.deckhouse.io/namespace: namespace
   name: custom:manage:permission:mycustom:superresource:view
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

Хук мониторит `ClusterRoleBinding` и при создании биндинга ходит по всем manage-ролям, чтобы найти все объединенные в них роли с помощью проверки правила агрегации. Затем он берёт пространство имён из лейбла `rbac.deckhouse.io/namespace` и создает use-роль в этом пространстве имён.

### Расширение существующих use-ролей

Если ресурс принадлежит пространству имён, необходимо расширить use-роль вместо manage-роли. Разница лишь в лейблах и имени:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-kubernetes-as: user
     rbac.deckhouse.io/kind: use
   name: custom:use:capability:mycustom:superresource:view
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

Эта роль дополнит роль `d8:use:role:user:kubernetes`.
