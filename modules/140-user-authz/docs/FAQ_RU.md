---
title: "Модуль user-authz: FAQ"
---

## Как создать пользователя?

[Создание пользователя](usage.html#создание-пользователя).

<div style="height: 0;" id="как-ограничить-права-пользователю-конкретными-пространствами-имён-устаревшая-ролевая-модель"></div>

## Как ограничить права пользователю конкретными пространствами имён?

Чтобы ограничить права пользователя конкретными пространствами имён в экспериментальной ролевой модели, используйте в `RoleBinding` [use-роль](./#use-роли) с соответствующим уровнем доступа. [Пример...](usage.html#пример-назначения-административных-прав-пользователю-в-рамках-пространства-имён).

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

<div style="height: 0;" id="как-перевести-кастомные-роли-на-новую-схему-в-dp-178"></div>

## Как перевести кастомные роли на новую схему?

Вместе с [переименованием ролей](./#миграция-на-новые-имена-ролей-в-dp-178) меняется схема лейблов, по которым агрегируются права.

{% alert level="warning" %}
Заморозьте кастомные роли старой схемы, пока встроенные capabilities ещё несут старые лейблы. Роль, которая уже потеряла права, заморозить вместе с ними нельзя.
{% endalert %}

В новой схеме у встроенных capabilities нет лейблов `rbac.deckhouse.io/kind: manage` и `rbac.deckhouse.io/kind: use`.
Кастомная роль, селектор агрегации которой требует один из этих лейблов (например, `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<подсистема>-as`), больше не находит ни одной из них.
Контроллер агрегации Kubernetes оставляет такую роль без правил, и её субъекты теряют права.
Псевдонимы совместимости для кастомных ролей не создаются.

В новой ролевой модели также меняется список подсистем.
Её роли собирают только линейки `system`, `namespace` и `project` и линейки подсистем `iam`, `security`, `cluster`, `delivery`, `network`, `storage`, `observability` и `managed-services`, и модули ставят на свои capabilities только лейблы этих линеек.
Селектор, который требует лейбл агрегации другой линейки, тогда тоже не находит ни одной встроенной capability.
Это, например, лейбл `rbac.deckhouse.io/aggregate-to-all-as`, лейбл подсистемы `deckhouse`, `infrastructure`, `kubernetes` или `networking` или лейбл подсистемы модуля, который переходит в одну из новых подсистем, например `rbac.deckhouse.io/aggregate-to-virtualization-as`.
Исключение составляет линейка, которую несёт кастомная capability, потому что селектор и после обновления получает правила этой capability. Кастомная capability здесь означает объект ClusterRole без лейбла `heritage: deckhouse` и без поля `aggregationRule`.
Исключение составляет и линейка собственной подсистемы роли из её лейбла `rbac.deckhouse.io/subsystem`, потому что её несут кастомные capabilities этой подсистемы.
Ни одно из исключений не относится к линейкам `all`, `deckhouse`, `infrastructure`, `kubernetes` и `networking`, которые не считаются кастомными.

Такие роли перечисляет алерт `D8UserAuthzLegacyRBACv2CustomRoleFound`.
Он срабатывает на каждый объект ClusterRole без лейбла `heritage: deckhouse`, в поле `aggregationRule` которого есть селектор, требующий лейбл `rbac.deckhouse.io/kind: manage` или `use` или такой лейбл агрегации, независимо от имени роли.
Он срабатывает и на объект ClusterRole без лейбла `heritage: deckhouse`, у которого есть поле `aggregationRule` и лейблы `rbac.deckhouse.io/kind: manage` и `rbac.deckhouse.io/use-role`, какие бы селекторы у него ни были.
Для каждого объекта ClusterRoleBinding на такую роль Deckhouse создаёт объекты RoleBinding в неймспейсах модулей. В новой ролевой модели он создаёт их только для ролей с лейблом `rbac.deckhouse.io/scope: system` или `subsystem`, поэтому обновление удаляет объекты RoleBinding такой роли, см. [Заморозка роли](#заморозка-роли).
Чтобы получить список таких ролей, используйте команду:

```shell
d8 k get clusterroles -o json | jq -r '
  def lineages: [((.matchLabels // {}) | keys[]), (.matchExpressions[]? | select(.operator == "In" or .operator == "Exists") | .key)]
    | map(capture("^rbac\\.deckhouse\\.io/aggregate-to-(?<lineage>.+)-as$").lineage);
  ([.items[] | select(.metadata.labels.heritage != "deckhouse" and .aggregationRule == null) | (.metadata.labels // {}) | keys[]
        | capture("^rbac\\.deckhouse\\.io/aggregate-to-(?<lineage>.+)-as$").lineage]) as $carried
  | .items[]
  | select(.metadata.labels.heritage != "deckhouse" and .aggregationRule != null)
  | (.metadata.labels["rbac.deckhouse.io/subsystem"] // "") as $own
  | select(
      (
        .metadata.labels["rbac.deckhouse.io/kind"] == "manage"
        and ((.metadata.labels["rbac.deckhouse.io/use-role"] // "") != "")
      )
      or (
        [
          .aggregationRule.clusterRoleSelectors[]?
          | (.matchLabels["rbac.deckhouse.io/kind"] // empty),
            (.matchExpressions[]? | select(.key == "rbac.deckhouse.io/kind" and .operator == "In") | .values[])
        ]
        | any(IN("manage", "use"))
      )
      or (
        [
          .aggregationRule.clusterRoleSelectors[]? | lineages[]
          | select((IN("system", "namespace", "project", "iam", "security", "cluster", "delivery", "network", "storage", "observability", "managed-services") | not)
              and (IN("all", "deckhouse", "infrastructure", "kubernetes", "networking") or (. != $own and (IN($carried[]) | not))))
        ]
        | length > 0
      )
    )
  | .metadata.name'
```

Соответствие старой и новой схем:

| Было (старая схема) | Стало (новая схема) |
|---------------------|---------------------|
| Произвольное имя роли (например, `custom:manage:mycustom:manager`) | Обязательный префикс `d8:custom:`, а у роли подсистемы ещё и имя подсистемы (например, `d8:custom:mycustom:manager`) |
| `rbac.deckhouse.io/kind: manage` или `use` на кастомной роли | `rbac.deckhouse.io/kind: custom-role` |
| `rbac.deckhouse.io/kind: manage` или `use` на кастомной capability | `rbac.deckhouse.io/kind: custom-capability`, имя с префиксом `d8:custom:` |
| `rbac.deckhouse.io/level: all \| subsystem \| module` | `rbac.deckhouse.io/scope: system \| subsystem \| namespace` |
| `rbac.deckhouse.io/aggregate-to-all-as: <уровень>` на кастомной capability | `rbac.deckhouse.io/aggregate-to-system-as: <уровень>` |
| `rbac.deckhouse.io/aggregate-to-all-as: <уровень>` на кастомной роли | Лейбла нет. Кастомную роль нельзя агрегировать в `d8:system:<уровень>`, поэтому привяжите её к субъектам `d8:system:<уровень>` через ClusterRoleBinding |
| Селектор агрегации: `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<подсистема>-as: <уровень>` | Только `rbac.deckhouse.io/aggregate-to-<подсистема>-as: <уровень>` |
| Лейблы подсистем `deckhouse`, `infrastructure` и `kubernetes` (`rbac.deckhouse.io/aggregate-to-deckhouse-as` и другие) | Один лейбл подсистемы `cluster`, `rbac.deckhouse.io/aggregate-to-cluster-as`, или селекторы по `rbac.deckhouse.io/capability`, чтобы роль осталась такой же узкой, как раньше, см. [Селекторы по линейке, которую не собирает новая ролевая модель](#селекторы-по-линейке-которую-не-собирает-новая-ролевая-модель) |
| Лейбл подсистемы `networking`, `rbac.deckhouse.io/aggregate-to-networking-as` | `rbac.deckhouse.io/aggregate-to-network-as` |
| Модули `user-authz`, `user-authn` и `multitenancy-manager` с лейблами подсистем `security` и `deckhouse` | Лейбл подсистемы `iam`, `rbac.deckhouse.io/aggregate-to-iam-as` |
| Лейбл подсистемы модуля, который переходит в одну из новых подсистем, например `rbac.deckhouse.io/aggregate-to-virtualization-as` | Селекторы по `rbac.deckhouse.io/capability`, см. [Селекторы по линейке, которую не собирает новая ролевая модель](#селекторы-по-линейке-которую-не-собирает-новая-ролевая-модель) |
| Селектор для use-прав: `rbac.deckhouse.io/kind: use` + `rbac.deckhouse.io/aggregate-to-kubernetes-as: <уровень>` | `rbac.deckhouse.io/aggregate-to-namespace-as: <уровень>` |
| Селектор по модулю: `rbac.deckhouse.io/kind: manage` + `module: <модуль>` | `rbac.deckhouse.io/scope: system` + `module: <модуль>` |

Имена встроенных capabilities тоже меняются (без псевдонимов):

* `d8:manage:permission:module:<модуль>:view|edit` → `d8:system-capability:<модуль>:view|edit`;
* `d8:use:capability:module:<модуль>:view|edit` → `d8:namespace-capability:<модуль>:view|edit`;
* `d8:manage:permission:subsystem:<подсистема>:manage_resources|view_resources` → `d8:subsystem-capability:<подсистема>:manage_resources|view_resources`, где подсистема `kubernetes` становится `cluster`, а `networking` становится `network`;
* `d8:use:capability:kubernetes:<имя>` → `d8:namespace-capability:kubernetes:<имя>`.

Селекторы агрегации работают по лейблам, а не по именам. Прямые привязки к capabilities использовать не следует.

### Порядок миграции

Миграция проходит в два этапа.
Сначала, пока встроенные capabilities несут старые лейблы, заморозьте кастомные роли и добавьте новые лейблы кастомным capabilities. Замороженная роль сохраняет свои права при обеих схемах, а capability с новым лейблом остаётся в ролях, которые она расширяет.
Затем, когда встроенные capabilities получат новые лейблы, перейдите на роли новой схемы. Явные привязки, которые создаёт заморозка, указывают на роли `d8:use:role:<LEVEL>`. Новая схема сохраняет их только как устаревшие псевдонимы, и Deckhouse не устанавливает обновление, которое их удаляет, пока такие привязки есть.
После обновления перенесите их на `d8:namespace:<LEVEL>`.

#### Заморозка роли

Заморозьте каждую роль, которую алерт `D8UserAuthzLegacyRBACv2CustomRoleFound` перечисляет из-за её селекторов.
Соблюдайте порядок шагов.
Когда роль с лейблом `rbac.deckhouse.io/kind: manage` теряет поле `aggregationRule`, Deckhouse удаляет объекты RoleBinding, которые он автоматически создал для субъектов этой роли. Он удаляет и такие объекты ролей, в которые эта роль агрегируется, в неймспейсах, к которым ведёт только она.

1. Явно привяжите роль `d8:use:role:<LEVEL>` в неймспейсах автоматических объектов RoleBinding, к которым ведёт роль.

   Для каждого объекта ClusterRoleBinding на роль с лейблами `rbac.deckhouse.io/kind: manage` и `rbac.deckhouse.io/use-role: <LEVEL>` Deckhouse создаёт RoleBinding на `d8:use:role:<LEVEL>` с теми же субъектами в каждом неймспейсе, который указан в лейбле `rbac.deckhouse.io/namespace` у агрегируемых ею ролей.
   У этих объектов RoleBinding есть лейблы `heritage: deckhouse` и `rbac.deckhouse.io/automated: "true"` и аннотация `rbac.deckhouse.io/related-with: <CLUSTERROLEBINDING_NAME>`.
   Следующие команды копируют каждый из них для объектов ClusterRoleBinding на роль `BOUND_ROLE`. У копии тот же неймспейс, те же поля `roleRef` и `subjects` и аннотация `rbac.deckhouse.io/related-with`, префикс имени `explicit:` и нет этих лейблов:

   ```shell
   BOUND_ROLE=<ROLE_NAME>
   d8 k get clusterrolebindings -o json > clusterrolebindings.json
   d8 k get rolebindings -A -l heritage=deckhouse,rbac.deckhouse.io/automated=true -o json > automated-rolebindings.json
   jq --arg role "$BOUND_ROLE" --slurpfile crbs clusterrolebindings.json '
     [$crbs[0].items[] | select(.roleRef.kind == "ClusterRole" and .roleRef.name == $role) | .metadata.name] as $names
     | {apiVersion: "v1", kind: "List", items: [.items[]
         | select(.metadata.annotations["rbac.deckhouse.io/related-with"] | IN($names[]))
         | {apiVersion, kind, metadata: {name: ("explicit:" + .metadata.name), namespace: .metadata.namespace,
             annotations: {"rbac.deckhouse.io/related-with": .metadata.annotations["rbac.deckhouse.io/related-with"]}},
            roleRef, subjects}]}' \
     automated-rolebindings.json > explicit-rolebindings.json
   d8 k create -f explicit-rolebindings.json
   ```

   Выполните команды, указав в `BOUND_ROLE` имя роли, если у роли есть лейбл `rbac.deckhouse.io/use-role`.
   Затем выполните их ещё раз для каждой роли с лейблом `rbac.deckhouse.io/kind: manage`, в которую агрегируется эта роль, какие бы лейблы ни были у самой роли. Например, роль `d8:manage:all:<LEVEL>` агрегирует роли с лейблом `rbac.deckhouse.io/aggregate-to-all-as: <LEVEL>`. Автоматические объекты RoleBinding такой роли покрывают и неймспейсы, к которым ведёт только эта роль.

1. Если роль агрегируется в другую роль, например в `d8:manage:all:<LEVEL>`, привяжите саму роль к субъектам объектов ClusterRoleBinding на эту другую роль.
   Встроенные роли новой схемы выбирают другие лейблы, поэтому после обновления эти субъекты сохраняют права замороженной роли только через такую привязку:

   ```shell
   OUTER_ROLE=<OTHER_ROLE_NAME>
   jq --arg outer "$OUTER_ROLE" --arg role "<ROLE_NAME>" '
     {apiVersion: "v1", kind: "List", items: [.items[]
       | select(.roleRef.kind == "ClusterRole" and .roleRef.name == $outer)
       | {apiVersion, kind, metadata: {name: ("explicit:" + $role + ":" + .metadata.name)},
          roleRef: {apiGroup: "rbac.authorization.k8s.io", kind: "ClusterRole", name: $role}, subjects}]}' \
     clusterrolebindings.json > explicit-clusterrolebindings.json
   d8 k create -f explicit-clusterrolebindings.json
   ```

1. Удалите у роли поле `aggregationRule`, сохранив поле `rules`:

   ```shell
   d8 k patch clusterrole <ROLE_NAME> --type=json -p '[{"op": "remove", "path": "/aggregationRule"}]'
   ```

   Kubernetes не обновляет правила роли без поля `aggregationRule`, поэтому роль сохраняет права, которые есть у неё в этот момент, какие бы лейблы ни несли встроенные capabilities.
   Deckhouse удаляет автоматические объекты RoleBinding этой роли, а явные объекты из первого шага сохраняют доступ в этих неймспейсах.
   Роль пропадает из алерта `D8UserAuthzLegacyRBACv2CustomRoleFound`.

Замените `<ROLE_NAME>` на имя роли и `<OTHER_ROLE_NAME>` на имя роли, в которую она агрегируется.

Если роль применяется из Git-репозитория (например, с помощью Argo CD или Flux), внесите то же изменение в её манифест.
Удалите в нём поле `aggregationRule` и перенесите в поле `rules` текущие правила роли из вывода команды `d8 k get clusterrole <ROLE_NAME> -o yaml`.
Иначе следующее применение манифеста вернёт поле `aggregationRule` и пустое поле `rules`.
После обновления вебхук `rbacv2-cluster-roles.deckhouse.io` отклоняет любое изменение объекта ClusterRole, который несёт лейбл агрегации встроенной линейки, например `rbac.deckhouse.io/aggregate-to-all-as`, без лейбла `rbac.deckhouse.io/kind: custom-capability`, в том числе применение его манифеста.
Замороженную роль с таким лейблом нельзя изменить, пока её не заменят ролью новой схемы, и её называет алерт `D8UserAuthzForeignAggregationLabel`.

Deckhouse не удаляет явные объекты RoleBinding и ClusterRoleBinding.
Когда вы отзываете доступ субъекта, удалите их вместе с объектом ClusterRoleBinding, для которого они скопированы. У RoleBinding его называет аннотация `rbac.deckhouse.io/related-with`, а имя ClusterRoleBinding заканчивается его именем.

Замороженная роль не получает права на ресурсы, добавленные позже, например на ресурсы нового модуля или новой capability с подходящими лейблами.
Добавьте такие права в её поле `rules` самостоятельно или замените роль ролью новой схемы.

Роль, селекторы которой не требуют ни одного из этих лейблов, сохраняет права после обновления, и алерт перечисляет её только из-за лейблов `rbac.deckhouse.io/kind: manage` и `rbac.deckhouse.io/use-role`.
Не замораживайте такую роль. Выполните только команды первого шага, указав в `BOUND_ROLE` имя роли, чтобы явные объекты RoleBinding сохранили доступ в неймспейсах модулей после обновления.
Роль остаётся в алерте, пока после обновления её не заменят ролью новой схемы.

#### Новый лейбл для capability

Добавьте лейбл каждой capability, которую перечисляет алерт `D8UserAuthzLegacyRBACv2CustomCapabilityFound`.
Capability с лейблами `rbac.deckhouse.io/kind: use` и `rbac.deckhouse.io/aggregate-to-kubernetes-as: <LEVEL>` расширяет роли `d8:use:role:<LEVEL>`.
В новой схеме эти роли выбирают лейбл `rbac.deckhouse.io/aggregate-to-namespace-as`. Добавьте его и сохраните старые лейблы, которые читают текущие роли:

```shell
d8 k label clusterrole <CAPABILITY_NAME> rbac.deckhouse.io/aggregate-to-namespace-as=<LEVEL>
```

Если capability применяется из Git-репозитория, добавьте лейбл и в её манифест.

После обновления замените capability на capability новой схемы.
Создайте `d8:custom:namespace-capability:<NAME>` с теми же правилами и лейблами `rbac.deckhouse.io/kind: custom-capability`, `rbac.deckhouse.io/scope: namespace` и `rbac.deckhouse.io/aggregate-to-namespace-as: <LEVEL>`, проверьте права субъектов и затем удалите старую capability.
После обновления команды приведены в разделе «Замена capability» этого FAQ.
Пока capability не заменена, вебхук `rbacv2-cluster-roles.deckhouse.io` отклоняет любое её изменение, потому что она несёт лейблы агрегации без `rbac.deckhouse.io/kind: custom-capability`, и применение её манифеста из Git-репозитория тоже завершается ошибкой.

#### Переход на роль новой схемы

Когда встроенные capabilities получат новые лейблы, замороженную роль можно заменить ролью новой схемы, которая снова собирает права:

1. Создайте роль с префиксом имени `d8:custom:`, лейблом `rbac.deckhouse.io/kind: custom-role` и новыми селекторами агрегации. Руководствуйтесь примерами «до и после» ниже. Селектор по лейблу агрегации линейки, которую не собирает новая ролевая модель, замените, как описано в разделе [Селекторы по линейке, которую не собирает новая ролевая модель](#селекторы-по-линейке-которую-не-собирает-новая-ролевая-модель).
1. Пересоздайте кастомные capabilities с лейблом `rbac.deckhouse.io/kind: custom-capability` и префиксом имени `d8:custom:`.
1. Для каждого объекта RoleBinding и ClusterRoleBinding, указывающего на замороженную роль, создайте объект с теми же субъектами, указывающий на новую роль. Поле `roleRef` неизменяемо, поэтому создайте новый объект, а не редактируйте старый.
1. Проверьте права субъектов командой `d8 k auth can-i --as <USER_NAME>`. Затем удалите старые объекты RoleBinding и ClusterRoleBinding, замороженную роль и старые capabilities. Явные объекты RoleBinding, созданные при заморозке, удалите, когда новая роль будет давать тот же доступ в их неймспейсах.

#### Селекторы по линейке, которую не собирает новая ролевая модель

Селектор по лейблу агрегации линейки выбирает capabilities, которые несут этот лейбл.
Например, `rbac.deckhouse.io/aggregate-to-virtualization-as: <LEVEL>` выбирает capabilities модуля `virtualization`.
В новой ролевой модели модули ставят на свои capabilities лейбл линейки подсистемы, в которой они находятся, и селектор по этой линейке выбирает capabilities всех модулей подсистемы, а это может быть намного больше, чем давала роль раньше.
Например, в подсистему `cluster` входят большинство модулей подсистем `deckhouse`, `infrastructure` и `kubernetes` и модуль `virtualization`.
Чтобы новая роль осталась такой же узкой, как старая, выбирайте сами capabilities по лейблу `rbac.deckhouse.io/capability`.
Его значение не зависит от подсистемы. Для системной capability это `system-capability.<MODULE_NAME>.<ACTION>`, для capability неймспейса это `namespace-capability.<MODULE_NAME>.<ACTION>`.

1. Когда встроенные capabilities получат новые лейблы, получите список capabilities модуля с уровнями ролей, которые их собирают:

   ```shell
   d8 k get clusterroles -l rbac.deckhouse.io/kind=capability,module=<MODULE_NAME> -o json | jq -r '
     .items[]
     | [.metadata.labels["rbac.deckhouse.io/capability"],
        (.metadata.labels | to_entries
          | map(select(.key | test("^rbac\\.deckhouse\\.io/aggregate-to-.+-as$")) | (.key | ltrimstr("rbac.deckhouse.io/aggregate-to-") | rtrimstr("-as")) + "=" + .value)
          | join(","))]
     | @tsv'
   ```

1. Оставьте capabilities, которые находил старый селектор. Селектор со значением `<LEVEL>` находил capabilities этого уровня и, через встроенные роли нижних уровней, capabilities этих уровней.
1. В новой роли укажите по одному селектору по лейблу `rbac.deckhouse.io/capability` для каждой из них. Чтобы выдать все системные capabilities модуля независимо от уровня, достаточно одного селектора по `rbac.deckhouse.io/scope: system` и `module: <MODULE_NAME>`.

Например, следующая роль неймспейса даёт в неймспейсе то же, что давал селектор по `rbac.deckhouse.io/aggregate-to-virtualization-as: user`, без других модулей подсистемы. Замените `<ACTION>` значениями из списка:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace:virtualization-user
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: namespace
    rbac.deckhouse.io/delegatable: "true"
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/capability: namespace-capability.virtualization.<ACTION>
    - matchLabels:
        rbac.deckhouse.io/capability: namespace-capability.virtualization.<ACTION>
rules: []
```

Роль системной области или подсистемы выбирает значения `system-capability.<MODULE_NAME>.<ACTION>` так же.

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
  aggregationRule:
    clusterRoleSelectors:
      - matchLabels:
          rbac.deckhouse.io/aggregate-to-cluster-as: manager
      - matchLabels:
          rbac.deckhouse.io/scope: system
          module: user-authn
  rules: []
  ```

Что изменилось:

- имя получило обязательный префикс `d8:custom:` и называет подсистему роли;
- `rbac.deckhouse.io/kind: manage` → `rbac.deckhouse.io/kind: custom-role`;
- `rbac.deckhouse.io/level: subsystem` → `rbac.deckhouse.io/scope: subsystem`;
- лейбл `rbac.deckhouse.io/aggregate-to-all-as` удалён. Кастомную роль нельзя агрегировать в `d8:system:<уровень>`, потому что лейбл `rbac.deckhouse.io/aggregate-to-system-as` может нести только кастомная capability. Чтобы выдать роль субъектам `d8:system:manager`, привяжите её к ним через ClusterRoleBinding;
- из селекторов агрегации убран лейбл `rbac.deckhouse.io/kind: manage`;
- выборка всех системных прав модуля теперь выполняется по `rbac.deckhouse.io/scope: system` + `module: <модуль>`;
- подсистемы `deckhouse` и `kubernetes` в новой ролевой модели входят в подсистему `cluster`, поэтому два их селектора стали одним. Он выбирает всю подсистему `cluster`, а она шире двух прежних. Чтобы роль осталась такой же узкой, как раньше, выбирайте вместо него capabilities нужных модулей по `rbac.deckhouse.io/capability`, см. [Селекторы по линейке, которую не собирает новая ролевая модель](#селекторы-по-линейке-которую-не-собирает-новая-ролевая-модель).

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
| `rbac.deckhouse.io/aggregate-to-all-as` | `<уровень>` | На capability переименован в `rbac.deckhouse.io/aggregate-to-system-as`, с кастомной роли удалён | Агрегация объекта в общесистемную роль (`d8:system:<уровень>`) |
| `rbac.deckhouse.io/aggregate-to-<подсистема>-as` | Использовался в селекторах вместе с `rbac.deckhouse.io/kind: manage` | Используется в селекторах сам по себе | Агрегация объекта в подсистемную роль указанного уровня |
| `rbac.deckhouse.io/aggregate-to-kubernetes-as` | `<уровень>` (для use-прав) | Переименован в `rbac.deckhouse.io/aggregate-to-namespace-as` | Агрегация объекта в namespace-роль (`d8:namespace:<уровень>`) |
| `rbac.deckhouse.io/namespace` | Неймспейс | Без изменений | Дополнительный неймспейс, в котором обладателям роли автоматически создаётся RoleBinding |
| `rbac.deckhouse.io/capability` | — | Уникальное имя capability (например, `system-capability.deckhouse.view`) | Машиночитаемый идентификатор встроенной capability |
| `rbac.deckhouse.io/deprecated` | — | `"true"` на ролях-псевдонимах | Роль устарела и будет удалена; переведите привязки на новую роль |
| `module` | Имя модуля | Без изменений | Принадлежность встроенного объекта модулю DP; удобен в селекторах агрегации вместе со `scope` |
| `heritage: deckhouse` | Признак объекта платформы | Без изменений | Устанавливать на кастомные объекты нельзя |

Аннотации на объектах ClusterRole (в старой схеме аннотации не использовались):

| Аннотация | Назначение |
|-----------|------------|
| `ru.meta.deckhouse.io/title`, `ru.meta.deckhouse.io/description` | Отображаемые название и описание роли или capability на русском языке (платформа ставит их на встроенные объекты; на кастомных можно указать свои) |
| `en.meta.deckhouse.io/title`, `en.meta.deckhouse.io/description` | То же на английском языке |
| `rbac.deckhouse.io/deprecated-replaced-by` | Появляется вместе с новой схемой и ставится на каждую роль, которая сохраняет прежнее имя для совместимости. Содержит имя новой роли, на которую нужно перевести привязки. Прежняя роль агрегирует те же capabilities, что и эта новая роль, поэтому даёт права новой роли, и они могут отличаться от прав, которые прежняя роль давала раньше. Прежние имена временные |

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
