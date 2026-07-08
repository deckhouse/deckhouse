---
title: "Модуль user-authz: FAQ"
---

## Как создать пользователя?

[Создание пользователя](usage.html#создание-пользователя).

<div style="height: 0;" id="как-ограничить-права-пользователю-конкретными-пространствами-имён-устаревшая-ролевая-модель"></div>

## Как ограничить права пользователю конкретными пространствами имён?

Чтобы ограничить права пользователя конкретными пространствами имён в экспериментальной ролевой модели, используйте в `RoleBinding` [namespace-роль](./#namespace-роли) с соответствующим уровнем доступа. [Пример...](usage.html#пример-назначения-административных-прав-пользователю-в-рамках-пространства-имён).

В текущей ролевой модели используйте параметры `namespaceSelector` или `limitNamespaces` (устарел) в кастомном ресурсе [ClusterAuthorizationRule](cr.html#clusterauthorizationrule).

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
* Опции `namespaceSelector` будут объединены так, что `Jane Doe` будет иметь доступ в пространства имён, помеченные лейблом `env` со значением `review`, `stage` или `prod`.

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
  name: d8:custom:subsystem:mycustom:manager
  labels:
    rbac.deckhouse.io/use-role: admin
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: mycustom
    rbac.deckhouse.io/aggregate-to-system-as: manager
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
    - matchLabels:
        rbac.deckhouse.io/scope: system
        module: user-authn
rules: []
```

В начале указаны лейблы для новой роли:

- показывает, какую namespace-роль хук должен использовать при создании `RoleBinding` в пространствах имён модулей:

  ```yaml
  rbac.deckhouse.io/use-role: admin
  ```

- показывает, что роль является кастомной (кастомные роли не определяют собственных правил, а только агрегируют capabilities):

  ```yaml
  rbac.deckhouse.io/kind: custom-role
  ```

  > Этот лейбл обязателен.

- показывает, что роль является ролью подсистемы, и обрабатываться будет соответственно:

  ```yaml
  rbac.deckhouse.io/scope: subsystem
  ```

- указывает подсистему, за которую отвечает роль:

  ```yaml
  rbac.deckhouse.io/subsystem: mycustom
  ```

- позволяет роли `d8:system:manager` агрегировать эту роль в себя:

  ```yaml
  rbac.deckhouse.io/aggregate-to-system-as: manager
  ```

Далее указаны селекторы, именно они реализуют агрегацию:

- агрегирует роль менеджера из подсистемы `deckhouse`:

  ```yaml
  rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
  ```

- агрегирует все системные (scope `system`) capabilities модуля user-authn:

  ```yaml
   rbac.deckhouse.io/scope: system
   module: user-authn
  ```

Таким образом роль получает права от подсистем `deckhouse`, `kubernetes` и от модуля user-authn.

Особенности:

* кастомные роли и capabilities должны иметь префикс имени `d8:custom:` (остальное пространство имён `d8:` зарезервировано за встроенными объектами Deckhouse);
* `RoleBinding` с namespace-ролью (`d8:namespace:<уровень>`) будут созданы в пространствах имён модулей агрегированных подсистем, уровень задаётся лейблом `rbac.deckhouse.io/use-role`.

### Расширение пользовательской роли

Например, в кластере появился новый кластерный (пример для manage-роли) CRD-объект — MySuperResource, и нужно дополнить собственную роль из примера выше правами на взаимодействие с этим ресурсом.

Первым делом нужно дополнить роль новым селектором:

```yaml
rbac.deckhouse.io/aggregate-to-mycustom-as: manager
```

Этот селектор позволит агрегировать capabilities к новой подсистеме через указание этого лейбла. После добавления нового селектора роль будет выглядеть так:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   name: d8:custom:subsystem:mycustom:manager
   labels:
     rbac.deckhouse.io/use-role: admin
     rbac.deckhouse.io/kind: custom-role
     rbac.deckhouse.io/scope: subsystem
     rbac.deckhouse.io/subsystem: mycustom
     rbac.deckhouse.io/aggregate-to-system-as: manager
 aggregationRule:
   clusterRoleSelectors:
     - matchLabels:
         rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
     - matchLabels:
         rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
     - matchLabels:
         rbac.deckhouse.io/scope: system
         module: user-authn
     - matchLabels:
         rbac.deckhouse.io/aggregate-to-mycustom-as: manager
 rules: []
 ```

 Далее нужно создать новую capability, в которой следует определить права для нового ресурса. Например, только чтение:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-mycustom-as: manager
     rbac.deckhouse.io/kind: custom-capability
   name: d8:custom:capability:mycustom:superresource:view
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

Capability дополнит своими правами роль подсистемы, дав права на просмотр нового объекта.

Особенности:

* кастомные capabilities должны иметь префикс имени `d8:custom:`; остальная часть имени не ограничена, но для читаемости лучше использовать этот стиль.

### Расширение существующих подсистемных ролей

Если необходимо расширить существующую роль, нужно выполнить те же шаги, что и в пункте выше, но изменив лейблы и название роли.

Пример для расширения роли менеджера из подсистемы `deckhouse` (`d8:subsystem:deckhouse:manager`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    rbac.deckhouse.io/kind: custom-capability
  name: d8:custom:capability:mycustommodule:superresource:view
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

Таким образом новая capability расширит роль `d8:subsystem:deckhouse:manager`.

### Расширение подсистемных ролей с добавлением нового пространства имён

Если необходимо добавить новое пространство имён (для создания в нём хуком `RoleBinding` с namespace-ролью), потребуется добавить лишь один лейбл:

```yaml
"rbac.deckhouse.io/namespace": namespace
```

Этот лейбл сообщает хуку, что в этом пространстве имён нужно создать `RoleBinding` с namespace-ролью:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
     rbac.deckhouse.io/kind: custom-capability
     rbac.deckhouse.io/namespace: namespace
   name: d8:custom:capability:mycustom:superresource:view
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

Хук мониторит `ClusterRoleBinding` и при создании биндинга ходит по всем системным и подсистемным ролям, чтобы найти все объединенные в них capabilities с помощью проверки правила агрегации. Затем он берёт пространство имён из лейбла `rbac.deckhouse.io/namespace` и создает `RoleBinding` с namespace-ролью в этом пространстве имён.

### Расширение существующих namespace-ролей

Если ресурс принадлежит пространству имён, необходимо расширить namespace-роль вместо системной/подсистемной. Разница лишь в лейблах и имени:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-namespace-as: user
     rbac.deckhouse.io/kind: custom-capability
   name: d8:custom:namespace-capability:mycustom:superresource:view
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

Эта capability дополнит роль `d8:namespace:user`.

### Создание собственной namespace- или проектной роли

Иногда встроенная лестница уровней не подходит: например, нужна роль «разработчик» — просмотр всего пространства имён плюс чтение логов, но без права менять квоты или RBAC. Такая роль собирается из готовых capabilities, без написания RBAC-правил вручную.

Правила для собственных ролей:

- имя должно начинаться с `d8:custom:` (например, `d8:custom:namespace:developer`);
- роль должна иметь лейбл `rbac.deckhouse.io/kind: custom-role`;
- роль **не может содержать собственных правил** (`rules`) — только агрегировать capabilities через `aggregationRule`. Права описываются в отдельных capabilities — так состав роли всегда прозрачен;
- нельзя в одной роли агрегировать capabilities пользовательских областей (`namespace`, `project`) вместе с административными (`system`, подсистемы) — такая роль будет отклонена.

Пример: роль, включающая всё, что умеет `d8:namespace:viewer`, плюс одну конкретную capability (подключение к подам), выбранную адресно по её уникальному лейблу `rbac.deckhouse.io/capability`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace:developer
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: namespace
  annotations:
    custom.meta.deckhouse.io/title: "Разработчик"
    custom.meta.deckhouse.io/description: "Просмотр ресурсов и подключение к подам, без управления квотами и RBAC"
aggregationRule:
  clusterRoleSelectors:
    # Всё, что входит в уровень viewer namespace-линейки.
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-namespace-as: viewer
    # Плюс одна конкретная capability, выбранная по её уникальному имени.
    - matchLabels:
        rbac.deckhouse.io/capability: "namespace-capability.kubernetes.access_terminal"
rules: []
```

Если готовой capability с нужными правами нет, создайте собственную (`custom-capability` может содержать правила) и добавьте в `aggregationRule` роли селектор по её лейблу `rbac.deckhouse.io/capability` (в примере ниже — `matchLabels: {rbac.deckhouse.io/capability: "custom.logs-reader"}`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace-capability:logs-reader
  labels:
    rbac.deckhouse.io/kind: custom-capability
    rbac.deckhouse.io/capability: "custom.logs-reader"
rules:
  - apiGroups: [""]
    resources: ["pods/log"]
    verbs: ["get", "list"]
```

Список всех доступных capabilities и их уникальных имён:

```shell
d8 k get clusterroles -l rbac.deckhouse.io/kind=capability \
  -o custom-columns='NAME:.metadata.name,CAPABILITY:.metadata.labels.rbac\.deckhouse\.io/capability'
```

Готовую роль назначают так же, как встроенную: через `RoleBinding` в пространстве имён или через [ProjectRoleBinding](../multitenancy-manager/cr.html#projectrolebinding) на весь проект (для проектных ролей используйте `rbac.deckhouse.io/scope: project` и агрегируйте `aggregate-to-project-as`). Назначить её через `ClusterRoleBinding` нельзя — как и встроенные роли этих областей.

> Собрать такую роль можно и без YAML — мастером выдачи доступа в веб-интерфейсе Deckhouse Console: он показывает доступные capabilities, собирает из них роль и сразу создаёт нужную привязку.

## Как переименовать встроенную роль?

Изменять права встроенных ролей нельзя, но можно изменить их отображаемое название и описание — например, чтобы в интерфейсе они назывались в принятых в компании терминах. Для этого добавьте на роль аннотации `custom.meta.deckhouse.io/title` и `custom.meta.deckhouse.io/description`:

```shell
d8 k annotate clusterrole d8:namespace:admin \
  custom.meta.deckhouse.io/title='Администратор команды' \
  custom.meta.deckhouse.io/description='Управление ресурсами и доступом в пространстве имён команды'
```

Это единственное изменение, которое разрешено вносить в объекты с префиксом `d8:` (кроме `d8:custom:*`): попытка изменить правила, агрегацию или лейблы встроенной роли будет отклонена.

## Как узнать, у кого есть доступ к ресурсу?

В Enterprise Edition при включённом режиме мультитенантности ([`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)) доступен обратный запрос к авторизации — ресурс `WhoCan`. Он отвечает на вопрос «кто может выполнить действие X над ресурсом Y?» и возвращает список пользователей, групп и ServiceAccount'ов:

```shell
d8 k create -o yaml -f - <<EOF
apiVersion: authorization.deckhouse.io/v1alpha1
kind: WhoCan
metadata:
  name: who-can-create-networkpolicies
spec:
  resourceAttributes:
    namespace: my-namespace
    verb: create
    group: networking.k8s.io
    resource: networkpolicies
EOF
```

Ответ возвращается в поле `status` (`users`, `groups`, `serviceAccounts`) сразу в выводе команды; объект нигде не сохраняется.

Право создавать `WhoCan`-запросы даёт кластерная роль `d8:user-authz:who-can-checker`. Она намеренно никому не выдана по умолчанию: результат запроса раскрывает субъектов доступа во всех пространствах имён, поэтому выдавайте её только доверенным администраторам через `ClusterRoleBinding`.
