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

## Как перевести кастомные роли на новую схему в DKP 1.78?

{% alert level="warning" %}
Этот раздел описывает [переименование ролевой модели](./#миграция-на-новые-имена-ролей-в-dkp-178), которое вступит в силу в DKP 1.78. До DKP 1.78 кастомные роли и capabilities продолжают работать по старой схеме.
{% endalert %}

Вместе с переименованием ролей ([соответствие имён](./#миграция-на-новые-имена-ролей-в-dkp-178)) в DKP 1.78 изменится схема лейблов, используемых для агрегации прав.

После обновления до DKP 1.78 кастомные роли, созданные по старой схеме, **перестанут собирать права**: встроенные capabilities получат новые лейблы, и старые селекторы агрегации (например, `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<подсистема>-as`) больше не будут их находить. Псевдонимы совместимости для кастомных ролей и capabilities не создаются — их нужно обновить вручную.

Соответствие старой и новой схем:

| Было (старая схема) | Стало (новая схема) |
|---------------------|---------------------|
| Произвольное имя роли (например, `custom:manage:mycustom:manager`) | Обязательный префикс `d8:custom:` (например, `d8:custom:subsystem:mycustom:manager`) |
| `rbac.deckhouse.io/kind: manage` или `use` на кастомной роли | `rbac.deckhouse.io/kind: custom-role` |
| `rbac.deckhouse.io/kind: manage` или `use` на кастомной capability | `rbac.deckhouse.io/kind: custom-capability`, имя с префиксом `d8:custom:` |
| `rbac.deckhouse.io/level: all \| subsystem \| module` | `rbac.deckhouse.io/scope: system \| subsystem \| namespace` |
| `rbac.deckhouse.io/aggregate-to-all-as: <уровень>` | `rbac.deckhouse.io/aggregate-to-system-as: <уровень>` |
| Селектор агрегации: `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<подсистема>-as: <уровень>` | Только `rbac.deckhouse.io/aggregate-to-<подсистема>-as: <уровень>` |
| Селектор для use-прав: `rbac.deckhouse.io/kind: use` + `rbac.deckhouse.io/aggregate-to-kubernetes-as: <уровень>` | `rbac.deckhouse.io/aggregate-to-namespace-as: <уровень>` |
| Селектор по модулю: `rbac.deckhouse.io/kind: manage` + `module: <модуль>` | `rbac.deckhouse.io/scope: system` + `module: <модуль>` |

Имена встроенных capabilities также изменятся (без псевдонимов):

* `d8:manage:permission:module:<модуль>:view|edit` → `d8:system-capability:<модуль>:view|edit`;
* `d8:use:capability:module:<модуль>:view|edit` → `d8:namespace-capability:<модуль>:view|edit`.

Селекторы агрегации работают по лейблам, а не по именам, поэтому при миграции достаточно обновить селекторы. Прямые привязки к capabilities использовать не следует.

### Порядок миграции

После обновления до DKP 1.78 выполните следующее:

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
