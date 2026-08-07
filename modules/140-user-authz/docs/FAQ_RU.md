---
title: "Модуль user-authz: FAQ"
---

## Как создать пользователя?

[Создание пользователя](usage.html#создание-пользователя).

<div style="height: 0;" id="как-ограничить-права-пользователю-конкретными-пространствами-имён-устаревшая-ролевая-модель"></div>

## Как ограничить права пользователю конкретными пространствами имён?

Чтобы ограничить права пользователя конкретными пространствами имён в основной ролевой модели, используйте в `RoleBinding` [namespace-роль](./#namespace-роли) с соответствующим уровнем доступа. [Пример...](usage.html#пример-назначения-административных-прав-пользователю-в-рамках-пространства-имён).

В устаревшей ролевой модели используйте параметры `namespaceSelector` или `limitNamespaces` (устарел) в кастомном ресурсе [ClusterAuthorizationRule](cr.html#clusterauthorizationrule).

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

## Можно ли использовать устаревшую и основную ролевые модели одновременно?

Да. Обе модели в итоге сводятся к стандартному механизму RBAC Kubernetes, а RBAC — разрешающая модель: права из всех источников **суммируются**. Если действие разрешено хотя бы одним источником — `ClusterAuthorizationRule`, `AuthorizationRule`, `RoleBinding` на роль основной модели или `ProjectRoleBinding`, — оно будет разрешено. Ничего специально «переключать» не нужно: можно оставить существующие `ClusterAuthorizationRule` и постепенно добавлять привязки ролей основной модели.

Единственное исключение — режим мультитенантности ([`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)). Если у пользователя есть `ClusterAuthorizationRule` с ограничением пространств имён (`limitNamespaces` или `namespaceSelector`), это ограничение работает как **жёсткая граница**: запросы в пространства имён вне списка отклоняются, даже если там у пользователя есть `RoleBinding`. Подробнее — [в описании модуля](./#rolebinding-car). Если пользователю нужен комбинированный доступ, используйте `AuthorizationRule` вместо `ClusterAuthorizationRule` или не задавайте ограничение пространств имён в CAR.

## Как получить аналог ролей ClusterAdmin и SuperAdmin в основной модели?

Роли «одной сущностью», как `ClusterAdmin` и `SuperAdmin` устаревшей модели, в основной модели нет — она сознательно разделяет управление платформой (системные роли) и доступ к приложениям (namespace- и проектные роли). Эквивалент собирается из **двух привязок**: `ClusterRoleBinding` на системную роль и [ClusterProjectRoleBinding](../multitenancy-manager/cr.html#clusterprojectrolebinding) на проектную роль (она действует во всех проектах, включая создаваемые позже).

Приблизительное соответствие уровней:

| Роль устаревшей модели | Эквивалент в основной модели |
|---------------------|----------------------------------------|
| `User` | `d8:namespace:viewer` (через `RoleBinding` или `ProjectRoleBinding`) |
| `PrivilegedUser` | `d8:namespace:user` |
| `Editor` | `d8:namespace:manager` |
| `Admin` | `d8:namespace:admin` |
| `ClusterEditor` | `d8:system:manager` (примерно; область — платформа и системные пространства имён) |
| `ClusterAdmin` | `d8:system:manager` + `ClusterProjectRoleBinding` на `d8:project:admin` |
| `SuperAdmin` | `d8:system:superadmin` + `ClusterProjectRoleBinding` на `d8:project:superadmin` |

Пример для `ClusterAdmin` (группа `k8s-admins`):

```yaml
# Платформа: конфигурация модулей DKP, cluster-wide-ресурсы, системные пространства имён.
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: k8s-admins-platform
subjects:
  - kind: Group
    name: k8s-admins
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:system:manager
  apiGroup: rbac.authorization.k8s.io
---
# Приложения: администратор во всех пространствах имён всех проектов (включая будущие).
apiVersion: deckhouse.io/v1alpha3
kind: ClusterProjectRoleBinding
metadata:
  name: k8s-admins-projects
spec:
  subjects:
    - kind: Group
      name: k8s-admins
  roleRef:
    kind: ClusterRole
    name: d8:project:admin
```

Для `SuperAdmin` замените роли на `d8:system:superadmin` и `d8:project:superadmin`.

Особенности:

- При включённом [автоматическом создании проектов](../multitenancy-manager/configuration.html#parameters-allownamespaceswithoutprojects) каждое пользовательское пространство имён является проектом, поэтому пара «системная роль + `ClusterProjectRoleBinding`» покрывает и платформу, и все пользовательские пространства имён. Не покрывается только пространство имён `default` — оно не относится ни к проектам, ни к системным.
- Создать собственную роль «со всеми правами» (`apiGroups: ["*"], resources: ["*"], verbs: ["*"]`) не получится: такая роль даёт в том числе права на управление проектами и будет отклонена [встроенной защитой](./#встроенные-защиты-ролевой-модели). Если нужен именно неограниченный доступ ко всему API (вне ролевой модели платформы), используйте `ClusterRoleBinding` на встроенную роль Kubernetes `cluster-admin` — назначить её может только тот, у кого такие права уже есть.

## Как дать пользователю доступ только к ресурсам одного модуля?

Типовой запрос: пользователь в пространстве имён должен работать только с ресурсами одного модуля (например, только с виртуальными машинами), не видя остальных ресурсов (`Pod`, `Deployment` и т. п.).

Каждый модуль DKP поставляет отдельные capabilities на свои ресурсы, поэтому такой доступ выдаётся без написания RBAC-правил. Соберите [собственную роль](#создание-собственной-namespace--или-проектной-роли), агрегирующую только capabilities нужного модуля (селектор по лейблу `module`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace:virtualization-only
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: namespace
    rbac.deckhouse.io/delegatable: "true"   # Разрешает использовать роль в RoleBinding внутри проектов.
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/kind: capability
        rbac.deckhouse.io/scope: namespace
        module: virtualization
rules: []
```

Выдайте роль через `RoleBinding` в нужном пространстве имён или через [ProjectRoleBinding](../multitenancy-manager/cr.html#projectrolebinding) на весь проект. Пользователь получит доступ только к ресурсам модуля — стандартные ресурсы Kubernetes ему видны не будут.

Вне проектов то же самое можно сделать ещё проще — привязать capability модуля напрямую, без создания роли:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: virtualization-view
  namespace: my-namespace
subjects:
  - kind: User
    name: user@example.com
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:namespace-capability:virtualization:view
  apiGroup: rbac.authorization.k8s.io
```

Обратите внимание: внутри пространств имён **проектов** обычный `RoleBinding` может ссылаться только на роли, [доступные проекту](../multitenancy-manager/usage.html#какие-роли-доступны-в-rolebinding-внутри-проекта), — capabilities туда по умолчанию не входят, поэтому для проектов используйте вариант с собственной ролью (лейбл `rbac.deckhouse.io/delegatable: "true"` в примере выше как раз делает её доступной) либо `ProjectRoleBinding`.

## Как расширить роли или создать новую?

[Основная ролевая модель](./#основная-ролевая-модель) построена на принципе агрегации, она собирает более мелкие роли в более обширные,
тем самым предоставляя лёгкие способы расширения модели собственными ролями.

### Создание новой роли подсистемы

Предположим, что текущие подсистемы не подходят под ролевое распределение в компании и требуется создать новую [подсистему](./#подсистемы-ролевой-модели),
которая будет включать в себя роли из подсистемы `deckhouse`, подсистемы `kubernetes` и модуля user-authn.

Для решения этой задачи создайте следующую роль:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:mycustom:manager
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

* кастомные роли и capabilities должны иметь префикс имени `d8:custom:` (остальное пространство имён `d8:` зарезервировано за встроенными объектами Deckhouse). Имя должно согласовываться с объявленной областью: подсистемная роль — `d8:custom:<подсистема>:<имя>` (сегмент — сама подсистема, как в примере выше), namespace- или проектная роль — `d8:custom:namespace:<имя>` и `d8:custom:project:<имя>`, capability — `d8:custom:<область>-capability:<имя>`. Имя, расходящееся с лейблом `rbac.deckhouse.io/scope`, будет отклонено;
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
   name: d8:custom:mycustom:manager
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
     rbac.deckhouse.io/scope: subsystem
     rbac.deckhouse.io/subsystem: mycustom
     rbac.deckhouse.io/capability: "custom.subsystem-capability.mycustom.superresource_view"
   name: d8:custom:subsystem-capability:mycustom:superresource:view
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
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: deckhouse
    rbac.deckhouse.io/capability: "custom.subsystem-capability.deckhouse.superresource_view"
  name: d8:custom:subsystem-capability:deckhouse:superresource:view
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
     rbac.deckhouse.io/scope: subsystem
     rbac.deckhouse.io/subsystem: deckhouse
     rbac.deckhouse.io/namespace: namespace
   name: d8:custom:subsystem-capability:deckhouse:superresource:view
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

Хук смотрит только на объекты с лейблом `rbac.deckhouse.io/scope: system` или `subsystem`. Capability без этого лейбла всё так же отдаёт свои правила роли через агрегацию, но её лейбл `rbac.deckhouse.io/namespace` не будет прочитан, и `RoleBinding` в пространстве имён не появится.

### Расширение существующих namespace-ролей

Если ресурс принадлежит пространству имён, необходимо расширить namespace-роль вместо системной/подсистемной. Разница лишь в лейблах и имени:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-namespace-as: user
     rbac.deckhouse.io/kind: custom-capability
     rbac.deckhouse.io/scope: namespace
     rbac.deckhouse.io/capability: "custom.namespace-capability.mycustom.superresource_view"
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

## Как перевести кастомные роли на новую схему в DKP 1.78?

{% alert level="warning" %}
Кастомные роли и capabilities, в отличие от [встроенных ролей](./#устаревшие-имена-ролей), псевдонимов совместимости не получают. Пока не мигрирован каждый такой объект, обновление удерживается требованием релиза `legacyRBACv2CustomRolesCount`, а в кластере горит алерт `D8UserAuthzLegacyRBACv2CustomRoleFound`.
{% endalert %}

Вместе с переименованием ролей ([соответствие имён](./#устаревшие-имена-ролей)) изменилась и схема лейблов, используемых для агрегации прав.

Кастомные роли, созданные по старой схеме, после обновления **перестают собирать права**: встроенные capabilities получили новые лейблы, и старые селекторы агрегации (например, `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<подсистема>-as`) больше их не находят. Псевдонимы совместимости для кастомных ролей и capabilities не создаются — их нужно обновить вручную.

Соответствие старой и новой схем:

| Было (старая схема) | Стало (новая схема) |
|---------------------|---------------------|
| Произвольное имя роли (например, `custom:manage:mycustom:manager`) | Обязательный префикс `d8:custom:` (например, `d8:custom:mycustom:manager`) |
| `rbac.deckhouse.io/kind: manage` или `use` на кастомной роли | `rbac.deckhouse.io/kind: custom-role` |
| `rbac.deckhouse.io/kind: manage` или `use` на кастомной capability | `rbac.deckhouse.io/kind: custom-capability`, имя с префиксом `d8:custom:` |
| `rbac.deckhouse.io/level: all \| subsystem \| module` | `rbac.deckhouse.io/scope: system \| subsystem \| namespace` |
| `rbac.deckhouse.io/aggregate-to-all-as: <уровень>` | `rbac.deckhouse.io/aggregate-to-system-as: <уровень>` |
| Селектор агрегации: `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<подсистема>-as: <уровень>` | Только `rbac.deckhouse.io/aggregate-to-<подсистема>-as: <уровень>` |
| Селектор для use-прав: `rbac.deckhouse.io/kind: use` + `rbac.deckhouse.io/aggregate-to-kubernetes-as: <уровень>` | `rbac.deckhouse.io/aggregate-to-namespace-as: <уровень>` |
| Селектор по модулю: `rbac.deckhouse.io/kind: manage` + `module: <модуль>` | `rbac.deckhouse.io/scope: system` + `module: <модуль>` |

Имена встроенных capabilities также изменились (без псевдонимов):

* `d8:manage:permission:module:<модуль>:view|edit` → `d8:system-capability:<модуль>:view|edit`;
* `d8:use:capability:module:<модуль>:view|edit` → `d8:namespace-capability:<модуль>:view|edit`.

Селекторы агрегации работают по лейблам, а не по именам, поэтому при миграции достаточно обновить селекторы. Прямые привязки к capabilities использовать не следует.

Список всего, что ещё предстоит мигрировать:

```shell
d8 k get clusterroles -o json | jq -r '.items[] | select((.metadata.name | startswith("custom:")) and ((.metadata.labels["rbac.deckhouse.io/kind"] // "" | IN("manage", "use")) or ([.aggregationRule.clusterRoleSelectors[]?.matchLabels["rbac.deckhouse.io/kind"] // ""] | any(IN("manage", "use"))))) | .metadata.name'
```

### Порядок миграции

Выполните следующее:

1. Создайте новую версию кастомной роли с префиксом `d8:custom:`, лейблом `rbac.deckhouse.io/kind: custom-role` и новыми селекторами агрегации. Руководствуйтесь примерами «до и после» ниже.
1. Пересоздайте кастомные capabilities с лейблом `rbac.deckhouse.io/kind: custom-capability` и префиксом имени `d8:custom:`.
1. Пересоздайте объекты RoleBinding и ClusterRoleBinding, указывающие на старую роль, указав новые имена ролей в поле `roleRef`. Это поле является неизменяемым, поэтому существующие привязки необходимо удалить и создать заново.
1. После проверки корректности новых привязок удалите старые роли и capabilities.

### Примеры

#### Кастомная роль до и после

Пример конфигурации роли, объединяющей права подсистем `deckhouse` и `kubernetes` и модуля `user-authn`.

* Было (старая схема):

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

* Стало (новая схема):

  ```yaml
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  metadata:
    name: d8:custom:mycustom:manager
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

Что изменилось:

- имя получило обязательный префикс `d8:custom:`;
- `rbac.deckhouse.io/kind: manage` → `rbac.deckhouse.io/kind: custom-role`;
- `rbac.deckhouse.io/level: subsystem` → `rbac.deckhouse.io/scope: subsystem`;
- `rbac.deckhouse.io/aggregate-to-all-as` → `rbac.deckhouse.io/aggregate-to-system-as`;
- из селекторов агрегации убран лейбл `rbac.deckhouse.io/kind: manage`;
- выборка всех системных прав модуля теперь выполняется по `rbac.deckhouse.io/scope: system` + `module: <модуль>`.

#### Кастомная capability до и после

Пример конфигурации capability, которая даёт права на просмотр ресурса MySuperResource и агрегируется в роль из примера выше (в её поле `aggregationRule` должен быть селектор `rbac.deckhouse.io/aggregate-to-mycustom-as: manager`).

* Было (старая схема):

  ```yaml
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  metadata:
    name: custom:manage:permission:mycustom:superresource:view
    labels:
      rbac.deckhouse.io/kind: manage
      rbac.deckhouse.io/aggregate-to-custom-as: manager
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

* Стало (новая схема):

  ```yaml
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  metadata:
    name: d8:custom:capability:mycustom:superresource:view
    labels:
      rbac.deckhouse.io/kind: custom-capability
      rbac.deckhouse.io/aggregate-to-mycustom-as: manager
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

### Лейблы и аннотации: было и стало

Лейблы на объектах ClusterRole:

| Лейбл | Было | Стало | Назначение |
|-------|------|-------|------------|
| `rbac.deckhouse.io/kind` | `manage` или `use` | `custom-role` / `custom-capability` — для кастомных объектов; `role` / `capability` — у встроенных (зарезервированы) | Тип объекта ролевой модели. Обязателен: объекты без него не обрабатываются |
| `rbac.deckhouse.io/level` | `all` \| `subsystem` \| `module` | Удалён | Старый уровень роли; заменён лейблом `scope` |
| `rbac.deckhouse.io/scope` | — | `system` \| `subsystem` \| `namespace` | Область действия роли или capability |
| `rbac.deckhouse.io/subsystem` | Имя подсистемы | Без изменений | Подсистема роли; используется при `scope: subsystem` |
| `rbac.deckhouse.io/use-role` | Уровень use-роли | Уровень namespace-роли | Какая namespace-роль автоматически выдаётся обладателю системной/подсистемной роли в системных неймспейсах её модулей (через автоматически создаваемые объекты RoleBinding) |
| `rbac.deckhouse.io/aggregate-to-all-as` | `<уровень>` | Переименован в `rbac.deckhouse.io/aggregate-to-system-as` | Агрегация объекта в общесистемную роль (`d8:system:<уровень>`) |
| `rbac.deckhouse.io/aggregate-to-<подсистема>-as` | Использовался в селекторах вместе с `rbac.deckhouse.io/kind: manage` | Используется в селекторах сам по себе | Агрегация объекта в подсистемную роль указанного уровня |
| `rbac.deckhouse.io/aggregate-to-kubernetes-as` | `<уровень>` (для use-прав) | Переименован в `rbac.deckhouse.io/aggregate-to-namespace-as` | Агрегация объекта в namespace-роль (`d8:namespace:<уровень>`) |
| `rbac.deckhouse.io/namespace` | Неймспейс | Без изменений | Дополнительный неймспейс, в котором обладателям роли автоматически создаётся RoleBinding |
| `rbac.deckhouse.io/capability` | — | Уникальное имя capability (например, `system-capability.deckhouse.view`) | Машиночитаемый идентификатор встроенной capability |
| `rbac.deckhouse.io/deprecated` | — | `"true"` на ролях-псевдонимах | Роль устарела и будет удалена; переведите привязки на новую роль |
| `module` | Имя модуля | Без изменений | Принадлежность встроенного объекта модулю DKP; удобен в селекторах агрегации вместе со `scope` |
| `heritage: deckhouse` | Признак объекта платформы | Без изменений | Устанавливать на кастомные объекты нельзя |

Аннотации на объектах ClusterRole (в старой схеме аннотации не использовались):

| Аннотация | Назначение |
|-----------|------------|
| `ru.meta.deckhouse.io/title`, `ru.meta.deckhouse.io/description` | Отображаемые название и описание роли или capability на русском языке (платформа ставит их на встроенные объекты; на кастомных можно указать свои) |
| `en.meta.deckhouse.io/title`, `en.meta.deckhouse.io/description` | То же на английском языке |
| `rbac.deckhouse.io/deprecated-replaced-by` | Появится в DKP 1.78 вместе с новой схемой. Правила агрегации прежних ролей изменятся так, что роли продолжат давать те же права, что и соответствующие им новые — существующие привязки не сломаются. Однако сохраняются прежние роли только на один релиз DKP: за это время привязки нужно перевести на новые роли. Аннотация проставляется на каждой прежней роли и содержит имя новой роли, эквивалентной ей по правам, на которую следует мигрировать |

### Добавление кастомной capability (в новой схеме)

Capability — это обычный объект ClusterRole с правилами, который через лейбл агрегации автоматически включается в выбранную роль. В новой схеме кастомная capability создаётся следующим образом:

1. Определите, какую роль нужно расширить: namespace-роль, подсистемную, системную или кастомную.
1. Создайте ClusterRole с префиксом имени `d8:custom:` (для читаемости — `d8:custom:capability:<имя>:<ресурс>:<действие>`), лейблом `rbac.deckhouse.io/kind: custom-capability` и лейблом агрегации целевой роли:
   - `rbac.deckhouse.io/aggregate-to-namespace-as: <viewer|user|manager|admin|superadmin>` — в namespace-роль `d8:namespace:<уровень>`;
   - `rbac.deckhouse.io/aggregate-to-<подсистема>-as: <viewer|manager|superadmin>` — в подсистемную роль `d8:subsystem:<подсистема>:<уровень>`;
   - `rbac.deckhouse.io/aggregate-to-system-as: <viewer|manager|superadmin>` — в системную роль `d8:system:<уровень>`;
   - `rbac.deckhouse.io/aggregate-to-<имя своей подсистемы>-as: <уровень>` — в кастомную роль (такой селектор должен присутствовать в её поле `aggregationRule`).
1. Опишите права в `rules`.

Kubernetes агрегирует правила автоматически: сразу после создания capability её права появятся у всех обладателей целевой роли. Проверить результат можно командой `d8 k auth can-i --as <пользователь>` или посмотрев итоговые правила роли: `d8 k get clusterrole <роль> -o yaml`.

Примеры конфигурации доступны выше в подразделах «[Кастомная роль до и после](#кастомная-роль-до-и-после)» и «[Кастомная capability до и после](#кастомная-capability-до-и-после)».

## Как переименовать встроенную роль?

Изменять права встроенных ролей нельзя, но можно изменить их отображаемое название и описание — например, чтобы в интерфейсе они назывались в принятых в компании терминах. Для этого добавьте на роль аннотации `custom.meta.deckhouse.io/title` и `custom.meta.deckhouse.io/description`:

```shell
d8 k annotate clusterrole d8:namespace:admin \
  custom.meta.deckhouse.io/title='Администратор команды' \
  custom.meta.deckhouse.io/description='Управление ресурсами и доступом в пространстве имён команды'
```

Это единственное изменение, которое разрешено вносить в объекты с префиксом `d8:` (кроме `d8:custom:*`): попытка изменить правила, агрегацию или лейблы встроенной роли будет отклонена.

## Как узнать, у кого есть доступ к ресурсу?

При включённом режиме мультитенантности ([`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)) доступен обратный запрос к авторизации — ресурс `WhoCan`. Он отвечает на вопрос «кто может выполнить действие X над ресурсом Y?» и возвращает список пользователей, групп и ServiceAccount'ов:

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

## Как узнать, что разрешено конкретному пользователю, группе или ServiceAccount'у?

При включённом режиме мультитенантности ([`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)) доступен ресурс `SubjectAccessReport` — обратная сторона `WhoCan`. Он отвечает на вопрос «что разрешено этому субъекту» и сразу возвращает готовый отчёт: какие роли и через какие привязки выданы, какие действия и над какими ресурсами разрешены в кластере и в каждом пространстве имён, и откуда взялось каждое право.

```shell
d8 k create -o yaml -f - <<EOF
apiVersion: authorization.deckhouse.io/v1alpha1
kind: SubjectAccessReport
metadata:
  name: what-can-jane-do
spec:
  subject:
    kind: User
    name: jane@example.com
EOF
```

Поле `spec.subject.kind` принимает значения `User`, `Group` и `ServiceAccount` (для последнего обязательно укажите `spec.subject.namespace`). Если `spec.subject` не указан, отчёт строится для того, кто выполняет запрос.

Особенности отчёта:

- Группы пользователя определяются автоматически по каталогу [Group](../user-authn/cr.html#group), включая вложенные: если пользователь состоит в группе `B`, а `B` входит в `A`, права `A` тоже попадут в отчёт. Учтённые группы возвращаются в `status.subject.groups`. Группы, которых нет в каталоге (например, приходящие из внешнего провайдера аутентификации), можно передать в `spec.groups`.
- В каждом источнике права (`status.scopes[].resources[].sources[]`) есть поле `matchedBy`: по нему видно, выдано право лично субъекту или через группу — и можно посмотреть картину прав без учёта групп.
- Пространства имён с одинаковым доступом объединяются в одну секцию (`status.scopes[].namespaces`), поэтому проект с десятком пространств имён не превращается в десяток одинаковых таблиц.
- Права, выданные `ClusterRoleBinding`, показываются один раз в кластерной области (`status.scopes[].cluster: true`) — они действуют во всех пространствах имён.
- Ограничить отчёт конкретными пространствами имён можно через `spec.namespaces`.

Отчёт строится по данным RBAC и не учитывает ограничения admission-вебхуков: например, изменение и удаление системных ресурсов запрещено всем, кроме уровня `superadmin`. Такие места помечены в `status.scopes[].caveat`. Если права субъекта ограничены ресурсом `ClusterAuthorizationRule`, об этом сообщается в `status.notes`.

Право строить отчёт о **другом** субъекте даёт кластерная роль `d8:user-authz:subject-access-checker`. Как и `who-can-checker`, она намеренно никому не выдана по умолчанию: отчёт раскрывает полную карту прав субъекта, включая чужие пространства имён. Отчёт о самом себе доступен любому аутентифицированному пользователю и не требует дополнительных прав.

## Как пользователю увидеть список доступных ему пространств имён?

При включённом режиме мультитенантности ([`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)) список пространств имён фильтруется автоматически: команда `d8 k get namespaces` возвращает пользователю только те пространства имён, к которым у него есть доступ — по любому из механизмов (привязки ролей, `ProjectRoleBinding`/`ClusterProjectRoleBinding`, `ClusterAuthorizationRule`/`AuthorizationRule`). Пользователь не видит чужих пространств имён и не может по списку узнать об их существовании.

Тот же список отдаёт read-only ресурс `accessiblenamespaces` — его может запросить любой аутентифицированный пользователь **для самого себя**:

```shell
d8 k get accessiblenamespaces
```

Это удобно для скриптов и интерфейсов: не нужно перебирать пространства имён и проверять доступ к каждому.
