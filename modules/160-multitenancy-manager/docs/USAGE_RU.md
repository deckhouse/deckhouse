---
title: "Модуль multitenancy-manager: примеры использования"
---
{% raw %}

## Шаблоны для проектов доступные по умолчанию

В Deckhouse Kubernetes Platform есть набор шаблонов для создания проектов:

- `simple` — минимальный шаблон, создающий только пространство имён проекта (с возможностью задать дополнительные лейблы и аннотации). Используйте его, когда нужно лишь изолированное пространство имён, управляемое как проект, а доступ и ограничения настраиваются через [стандартные поля](#стандартные-поля-проекта) и [привязки ролей проекта](#предоставление-доступа-внутри-проекта).

  Описание шаблона [в GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/simple.yaml).

- `default` — шаблон для базовых сценариев использования проектов:
  - сетевая изоляция;
  - автоматические алерты и сбор логов;
  - выбор профиля безопасности.

    Описание шаблона [в GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/default.yaml).

- `secure` — включает все возможности шаблона `default`, а также дополнительные функции:
  - настройка допустимых для проекта UID/GID;
  - правила аудита обращения Linux-пользователей проекта к ядру;
  - сканирование запускаемых образов контейнеров на наличие известных уязвимостей (CVE).

  Описание шаблона [в GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/secure.yaml).

- `secure-with-dedicated-nodes` — включает все возможности шаблона `secure`, а также дополнительные функции:
  - определение селектора узла для всех подов в проекте: если под создан, селектор узла пода будет автоматически **заменён** на селектор узла проекта;
  - определение стандартных tolerations для всех подов в проекте: если под создан, стандартные значения tolerations **добавляются** к нему автоматически.

  Описание шаблона [в GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/secure-with-dedicated-nodes.yaml).

Шаблоны `default`, `secure` и `secure-with-dedicated-nodes` описаны в [структурированном виде](#структурированные-шаблоны) (`deckhouse.io/v1alpha2`); шаблон `simple` — минимальный устаревший (`v1alpha1`) шаблон.

Чтобы перечислить все доступные параметры для шаблона проекта, выполните команду:

```shell
d8 k get projecttemplates <ИМЯ_ШАБЛОНА_ПРОЕКТА> -o jsonpath='{.spec.parametersSchema.openAPIV3Schema}' | jq
```

## Создание проекта

1. Для создания проекта создайте ресурс [Project](cr.html#project) с указанием имени шаблона проекта в поле [.spec.projectTemplateName](cr.html#project-v1alpha3-spec-projecttemplatename).
1. Задайте [стандартные поля](#стандартные-поля-проекта) — [.spec.administrators](cr.html#project-v1alpha3-spec-administrators) и [.spec.quota](cr.html#project-v1alpha3-spec-quota), — которые теперь управляются непосредственно ресурсом Project независимо от шаблона.
1. В параметре [.spec.parameters](cr.html#project-v1alpha3-spec-parameters) ресурса Project укажите значения параметров для секции [.spec.parametersSchema.openAPIV3Schema](cr.html#projecttemplate-v1alpha1-spec-parametersschema-openapiv3schema) ресурса ProjectTemplate.

   Пример создания проекта с помощью ресурса [Project](cr.html#project) из `default` [ProjectTemplate](cr.html#projecttemplate) представлен ниже:

   ```yaml
   apiVersion: deckhouse.io/v1alpha3
   kind: Project
   metadata:
     name: my-project
   spec:
     description: This is an example from the Deckhouse documentation.
     projectTemplateName: default
     # Стандартные поля, управляемые самим ресурсом Project.
     administrators:
       - kind: Group
         name: k8s-admins
     quota:
       requests.cpu: "5"
       requests.memory: 5Gi
       requests.storage: 1Gi
       limits.cpu: "5"
       limits.memory: 5Gi
     # Параметры конкретного шаблона.
     parameters:
       networkPolicy: Isolated
       podSecurityProfile: Restricted
       extendedMonitoringEnabled: true
   ```

   {% endraw %}

   {% alert level="info" %}
   API ресурса Project обслуживается как `deckhouse.io/v1alpha3`. Старые манифесты `v1alpha1`/`v1alpha2` продолжают работать: webhook конвертации автоматически переносит `parameters.administrators` и `parameters.resourceQuota` в стандартные поля `.spec.administrators` и `.spec.quota`.
   {% endalert %}

   {% raw %}

1. Для проверки статуса проекта выполните команду:

   ```shell
   d8 k get projects my-project
   ```

   Успешно созданный проект должен отображаться в статусе `Deployed` (синхронизирован). Если отображается статус `Error` (ошибка), добавьте аргумент `-o yaml` к команде (например, `d8 k get projects my-project -o yaml`) для получения более подробной информации о причине ошибки.

### Правила именования проектов

Имя проекта одновременно является именем его основного пространства имён, поэтому при создании проекта проверяются следующие правила:

- имя не может начинаться с `d8-` и `kube-` — эти префиксы зарезервированы за системными пространствами имён;
- имя не может быть длиннее 61 символа;
- если существует проект `foo`, нельзя создать проект `foo-bar` — и наоборот, при существующем проекте `foo-bar` нельзя создать проект `foo`. Имена вида `<проект>-*` зарезервированы под [дополнительные пространства имён](#дополнительные-пространства-имён-проекта) проекта: без этого правила дополнительное пространство имён одного проекта могло бы совпасть по имени с чужим проектом.

## Дополнительные пространства имён проекта

Если приложению нужно несколько пространств имён (например, отдельное для кэша или очередей), добавьте их в проект ресурсом [ProjectNamespace](cr.html#projectnamespace). Ресурс создаётся **в основном пространстве имён проекта**; итоговое пространство имён получает имя `<имя проекта>-<spec.name>`:

```yaml
apiVersion: deckhouse.io/v1alpha3
kind: ProjectNamespace
metadata:
  name: cache
  namespace: my-project
spec:
  name: cache   # Будет создано пространство имён my-project-cache.
```

Проверить состав проекта можно по его статусу:

```shell
d8 k get project my-project -o jsonpath='{.status.namespaces}'
```

Правила работы с `ProjectNamespace`:

- Поле `spec.name` неизменяемо: чтобы переименовать пространство имён, удалите ресурс и создайте новый.
- Итоговое имя `<имя проекта>-<spec.name>` не может быть длиннее 63 символов (ограничение Kubernetes на имена пространств имён).
- Создавать `ProjectNamespace` можно только в основном пространстве имён проекта — «вложить» его в дополнительное пространство имён или чужой проект нельзя. Если пространство имён с таким именем уже существует и принадлежит другому проекту, запрос будет отклонён.
- При удалении ресурса `ProjectNamespace` удаляется его пространство имён; при удалении проекта — все его пространства имён.

### Что распространяется на дополнительные пространства имён

Автоматически действует во **всех** пространствах имён проекта (и в основном, и в дополнительных):

- **Доступ**: привязки [ProjectRoleBinding](cr.html#projectrolebinding) и [ClusterProjectRoleBinding](cr.html#clusterprojectrolebinding), включая автоматический доступ администраторов проекта. При добавлении нового пространства имён все существующие привязки разворачиваются в него без каких-либо действий со стороны пользователя.
- **Namespaced-объекты шаблона**: сетевая политика (`networkPolicy.mode: Isolated`) и настройка сбора логов (`logShipping`) создаются в каждом пространстве имён проекта. Сетевая изоляция при этом разрешает трафик между пространствами имён одного проекта.
- **Кластерные политики шаблона** (`OperationPolicy`, `SecurityPolicy` из `allowedUIDs`/`allowedGIDs`): выбирают пространства имён по лейблу `projects.deckhouse.io/project`, то есть покрывают весь проект.
- **Наследуемые лейблы**: профиль безопасности подов (`security.deckhouse.io/pod-policy`), расширенный мониторинг (`extended-monitoring.deckhouse.io/enabled`), сканирование уязвимостей (`security-scanning.deckhouse.io/enabled`) и лейбл шаблона (`projects.deckhouse.io/project-template`) синхронизируются с основного пространства имён на дополнительные. Синхронизация полная: если фичу выключили в шаблоне, лейбл снимется и с дополнительных пространств имён. Благодаря лейблу шаблона [правила доступности кластерных ресурсов](#выдача-кластерных-ресурсов-проектам) тоже действуют во всех пространствах имён проекта.

Остаётся только в **основном** пространстве имён:

- квота проекта (`ResourceQuota` из [`.spec.quota`](cr.html#project-v1alpha3-spec-quota));
- дополнительные лейблы и аннотации из `namespaceMetadata` шаблона;
- аннотации размещения на узлах (из полей `nodeSelector` и `tolerations` шаблона).

### Лейблы пространств имён проекта

| Лейбл | Основное | Дополнительные | Назначение |
|-------|:--------:|:--------------:|------------|
| `projects.deckhouse.io/project: <имя проекта>` | ✓ | ✓ | Принадлежность проекту — общий лейбл всех пространств имён проекта |
| `projects.deckhouse.io/project-namespace: <spec.name>` | — | ✓ | Признак дополнительного пространства имён (имя ресурса `ProjectNamespace`) |
| `projects.deckhouse.io/project-template: <имя шаблона>` | ✓ | ✓ | Шаблон проекта; по нему применяются правила доступности кластерных ресурсов |
| `heritage: multitenancy-manager` | ✓ | ✓ | Пространство имён управляется контроллером проектов; вручную его менять нельзя |
| `security.deckhouse.io/pod-policy`, `extended-monitoring.deckhouse.io/enabled`, `security-scanning.deckhouse.io/enabled` | ✓ | ✓ (наследуются) | Политики и фичи из шаблона проекта |

Общий лейбл `projects.deckhouse.io/project` позволяет выбирать пространства имён проекта обычным `get ns`:

```shell
# Все пространства имён проекта (основное + дополнительные):
d8 k get ns -l projects.deckhouse.io/project=my-project

# Только дополнительные:
d8 k get ns -l 'projects.deckhouse.io/project=my-project,projects.deckhouse.io/project-namespace'

# Только основное:
d8 k get ns -l 'projects.deckhouse.io/project=my-project,!projects.deckhouse.io/project-namespace'
```

## Автоматическое создание проекта для пространства имён

По умолчанию (параметр [`allowNamespacesWithoutProjects: true`](configuration.html#parameters-allownamespaceswithoutprojects)) пространство имён, созданное напрямую (например, `d8 k create ns my-app`), автоматически оборачивается в проект с тем же именем:

- проект создаётся без шаблона и помечается лейблом `multitenancy.deckhouse.io/project-managed-by-namespace: "true"`;
- источник истины — пространство имён: его лейблы и аннотации синхронизируются в параметры проекта; редактируйте и удаляйте именно пространство имён (при его удалении проект удаляется автоматически);
- редактировать спецификацию такого проекта вручную нельзя. Чтобы превратить его в обычный проект (например, назначить шаблон), снимите с проекта лейбл `multitenancy.deckhouse.io/project-managed-by-namespace` — после этого проект управляется как обычно.

Если параметр `allowNamespacesWithoutProjects` выключен, создание пространств имён вне проектов запрещено — попытка `d8 k create ns` будет отклонена с пояснением.

Существующее пространство имён также можно явно принять в управление проектом, пометив его аннотацией `projects.deckhouse.io/adopt`. Например:

1. Создайте новое пространство имён:

   ```shell
   d8 k create ns test
   ```

1. Пометьте его аннотацией:

   ```shell
   d8 k annotate ns test projects.deckhouse.io/adopt=""
   ```

1. Убедитесь, что проект создался:

   ```shell
   d8 k get projects
   ```

   В списке проектов появится новый проект, соответствующий пространству имён:

   ```shell
   NAME        STATE      PROJECT TEMPLATE   DESCRIPTION                                            AGE
   deckhouse   Deployed   virtual            This is a virtual project                              181d
   default     Deployed   virtual            This is a virtual project                              181d
   test        Deployed   empty                                                                     1m
   ```

Шаблон созданного проекта можно изменить на существующий.

{% endraw %}

{% alert level="warning" %}
Обратите внимание, что при смене шаблона может возникнуть конфликт ресурсов: если в чарте шаблона прописаны ресурсы, которые уже присутствуют в пространстве имён, то применить шаблон не получится.
{% endalert %}

{% raw %}

## Стандартные поля проекта

Администраторы проекта и квоты ресурсов больше не являются параметрами шаблона — это поля верхнего уровня ресурса [Project](cr.html#project), работающие с любым шаблоном (включая `simple` и проекты без шаблона):

- `.spec.administrators` — список субъектов (`kind: User` или `kind: Group` и `name`), получающих административный доступ к проекту. Контроллер реализует этот доступ через автоматически создаваемый [ProjectRoleBinding](cr.html#projectrolebinding) в пространстве имён проекта.
- `.spec.quota` — набор жёстких лимитов [ResourceQuota](https://kubernetes.io/docs/concepts/policy/resource-quotas/) (например, `requests.cpu`, `limits.memory`). Контроллер поддерживает `ResourceQuota` в пространстве имён проекта и сообщает текущее потребление в `.status.usage`.

```yaml
apiVersion: deckhouse.io/v1alpha3
kind: Project
metadata:
  name: my-project
spec:
  projectTemplateName: simple
  administrators:
    - kind: Group
      name: k8s-admins
  quota:
    requests.cpu: "5"
    requests.memory: 5Gi
    limits.cpu: "10"
    limits.memory: 10Gi
```

{% alert level="warning" %}
Объекты `ResourceQuota` и `AuthorizationRule`, описанные внутри шаблонов проектов, больше не отрисовываются: такие ресурсы теперь управляются исключительно через `.spec.quota` и `.spec.administrators`. Существующие шаблоны, в которых они объявлены, продолжают работать, но эти объекты отфильтровываются при рендеринге.
{% endalert %}

## Предоставление доступа внутри проекта

Чтобы предоставить доступ к пространствам имён проекта помимо администраторов проекта, используйте привязки ролей, которые ссылаются на кластерные роли и автоматически разворачиваются в нужные пространства имён проектов:

- [ProjectRoleBinding](cr.html#projectrolebinding) (пространство имён, короткое имя `prb`) — предоставляет роль в рамках **одного** проекта. Должен создаваться в главном пространстве имён проекта (имя которого совпадает с именем проекта). Контроллер создаёт `RoleBinding` в каждом пространстве имён этого проекта.
- [ClusterProjectRoleBinding](cr.html#clusterprojectrolebinding) (кластерный, короткое имя `cprb`) — предоставляет роль во **всех** невиртуальных проектах. Контроллер создаёт `RoleBinding` в каждом пространстве имён каждого проекта и сообщает количество затронутых проектов в `.status.boundProjects`.

`roleRef` должен ссылаться на `ClusterRole`, имя которого начинается с одного из разрешённых префиксов (`d8:project:`, `d8:namespace:`, `d8:project-capability:`, `d8:namespace-capability:`, `d8:custom:`). Описание ролей — [в документации модуля user-authz](../user-authz/).

При создании привязок действуют следующие проверки:

- **Защита от повышения привилегий**: создать привязку может только пользователь, у которого есть право привязывать (`bind`) указанную роль. Например, администратор проекта (`d8:project:admin`) может выдавать встроенные роли `d8:project:*` и `d8:namespace:*`, но не может выдать роль шире своих полномочий.
- Роль должна существовать: привязка к несуществующей роли отклоняется.
- `ServiceAccount` в качестве субъекта `ProjectRoleBinding` должен принадлежать пространству имён этого же проекта.
- Системные и подсистемные роли (`d8:system:*`, `d8:subsystem:*`), а также произвольные роли вне перечисленных префиксов через проектные привязки выдать нельзя.
- Роли с аннотацией `rbac.deckhouse.io/disabled-for-direct-use-in-projects: "true"` запрещены для выдачи в проектах. Эту аннотацию администратор кластера может поставить на роль, чтобы вывести её из употребления: существующие привязки продолжают работать, но новые не создаются. Если такую роль использует шаблон проекта, проект переходит в статус `Error` с пояснением в условии `TemplateRolesAllowed`.

Привязка `d8-administrators`, создаваемая контроллером из поля [`.spec.administrators`](cr.html#project-v1alpha3-spec-administrators), управляется только контроллером — редактировать её вручную нельзя. Чтобы изменить состав администраторов, измените поле `.spec.administrators` проекта.

### Какие роли доступны в RoleBinding внутри проекта

Кроме проектных привязок, внутри пространства имён проекта можно использовать и обычный `RoleBinding` — тогда роль действует только в этом одном пространстве имён. Но в проектах набор ролей, доступных для обычного `RoleBinding`, ограничен: разрешены только кластерные роли с лейблом `rbac.deckhouse.io/delegatable: "true"`. Из встроенных это роли `d8:namespace:*` и `d8:project:*`, а также роли уровней доступа устаревшей ролевой модели (`user-authz:user`, `user-authz:privileged-user`, `user-authz:editor`, `user-authz:admin`).

`RoleBinding` на любую другую кластерную роль (например, `cluster-admin`, системные роли или capabilities) в проекте будет отклонён с сообщением `references "<роль>" which is not available to project`. Это защита от обхода изоляции проекта через привязку к слишком широкой роли.

Чтобы использовать в проектах [собственную роль](../user-authz/faq.html#создание-собственной-namespace--или-проектной-роли), добавьте на неё лейбл `rbac.deckhouse.io/delegatable: "true"`:

```shell
d8 k label clusterrole d8:custom:namespace:developer rbac.deckhouse.io/delegatable=true
```

Ограничение действует только в пространствах имён «настоящих» проектов. На [автоматически обёрнутые](#автоматическое-создание-проекта-для-пространства-имён) пространства имён (с лейблом `multitenancy.deckhouse.io/project-managed-by-namespace`) оно не распространяется.

```yaml
---
apiVersion: deckhouse.io/v1alpha3
kind: ProjectRoleBinding
metadata:
  name: viewers
  namespace: my-project
spec:
  subjects:
    - kind: User
      name: viewer@example.com
  roleRef:
    kind: ClusterRole
    name: d8:project:viewer
---
apiVersion: deckhouse.io/v1alpha3
kind: ClusterProjectRoleBinding
metadata:
  name: platform-viewers
spec:
  subjects:
    - kind: Group
      name: platform
  roleRef:
    kind: ClusterRole
    name: d8:project:viewer
```

## Структурированные шаблоны

Начиная с API-версии `deckhouse.io/v1alpha2`, шаблон проекта описывается **структурированными полями** — вместо текстового Helm-шаблона вы декларативно указываете, какие настройки получат пространства имён проекта. Контроллер сам создаёт из этих полей нужные объекты (сетевые политики, политики безопасности, настройки сбора логов и т. д.) в каждом пространстве имён проекта и поддерживает их в актуальном состоянии.

Доступные поля (все — необязательные; полный справочник — [в описании ресурса](cr.html#projecttemplate)):

| Поле | Что настраивает |
|------|-----------------|
| `podSecurityStandard` | Профиль безопасности подов: `Privileged`, `Baseline` или `Restricted` |
| `networkPolicy.mode` | Сетевая изоляция: `Isolated` (трафик разрешён только внутри проекта и от системных компонентов платформы) или `NotRestricted` |
| `features.monitoring` | Расширенный мониторинг пространств имён проекта |
| `features.vulnerabilityScanning` | Сканирование образов контейнеров на уязвимости |
| `logShipping.clusterDestinationRef` | Сбор логов подов проекта в указанное хранилище (`ClusterLogDestination`) |
| `nodeSelector`, `tolerations` | Размещение подов проекта на выделенных узлах |
| `allowedUIDs`, `allowedGIDs` | Допустимые диапазоны UID/GID контейнеров проекта |
| `runtimeAudit.enabled` | Аудит обращений процессов проекта к ядру Linux |
| `namespaceMetadata.labels`, `namespaceMetadata.annotations` | Дополнительные лейблы и аннотации пространств имён проекта |
| `resources`, `grantPolicies` | [Выдача кластерных ресурсов проектам](#выдача-кластерных-ресурсов-проектам) |
| `parametersSchema.openAPIV3Schema` | Схема параметров, которые задаются при создании проекта |

Пример структурированного шаблона:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ProjectTemplate
metadata:
  name: my-template
spec:
  title: "Шаблон команды"
  description: "Изолированный проект с мониторингом"
  podSecurityStandard: Baseline
  networkPolicy:
    mode: Isolated
  features:
    monitoring: true
    vulnerabilityScanning: true
```

### Параметризация шаблона

Любое «листовое» значение структурированного поля можно сделать параметром: вместо конкретного значения укажите `{fromParam: <имя параметра>}` и объявите параметр в `parametersSchema`. Тогда каждый проект задаёт своё значение в `.spec.parameters`, а если значение не задано — используется `default` из схемы.

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ProjectTemplate
metadata:
  name: my-parametrized-template
spec:
  podSecurityStandard:
    fromParam: securityProfile
  networkPolicy:
    mode:
      fromParam: networkMode
  parametersSchema:
    openAPIV3Schema:
      type: object
      properties:
        securityProfile:
          type: string
          enum: [Baseline, Restricted]
          default: Baseline
        networkMode:
          type: string
          enum: [Isolated, NotRestricted]
          default: Isolated
```

Проект, использующий такой шаблон:

```yaml
apiVersion: deckhouse.io/v1alpha3
kind: Project
metadata:
  name: my-project
spec:
  projectTemplateName: my-parametrized-template
  parameters:
    securityProfile: Restricted
```

Ссылки `fromParam` проверяются при создании шаблона: ссылка на необъявленный параметр или параметр несовместимого типа (например, строковый параметр для булева поля) будет отклонена.

### Проверки шаблонов

- Шаблон, который используется хотя бы одним проектом, нельзя удалить.
- Изменение шаблона автоматически применяется ко всем проектам, созданным из него.
- Устаревшие шаблоны `deckhouse.io/v1alpha1` с текстовым полем `resourcesTemplate` (Helm-шаблонизация) продолжают работать, но признаны устаревшими — новые шаблоны создавайте в структурированном виде. Ресурсы `ResourceQuota` и `AuthorizationRule` из таких шаблонов отфильтровываются при рендеринге (см. [стандартные поля проекта](#стандартные-поля-проекта)).

## Создание собственного шаблона для проекта

Для создания своего шаблона:

1. Возьмите за основу один из шаблонов по умолчанию, например, `default`.
1. Скопируйте его в отдельный файл, например, `my-project-template.yaml` при помощи команды:

   ```shell
   d8 k get projecttemplates default -o yaml > my-project-template.yaml
   ```

1. Отредактируйте файл `my-project-template.yaml`: измените [структурированные поля](#структурированные-шаблоны) и схему входных параметров под свои задачи.
1. Измените имя шаблона в поле `.metadata.name`.
1. Примените полученный шаблон командой:

   ```shell
   d8 k apply -f my-project-template.yaml
   ```

1. Проверьте доступность нового шаблона с помощью команды:

   ```shell
   d8 k get projecttemplates <ИМЯ_НОВОГО_ШАБЛОНА>
   ```

{% endraw %}

## Использование лейблов для управления ресурсами

При создании ресурсов в `ProjectTemplate` можно использовать специальные лейблы для управления поведением `multitenancy-manager` при обработке этих ресурсов:

### Пропуск создания лейбла `heritage: multitenancy-manager`

По умолчанию все ресурсы, созданные из `ProjectTemplate`, получают лейбл `heritage: multitenancy-manager`.  
Он запрещают изменение ресурсов пользователями или любым контроллером, кроме `multitenancy-manager`.  
Если необходимо разрешить изменение ресурса (например, для совместимости с другими системами, или в случае реализации собственного контроля изменения создаваемых объектов), добавьте к ресурсу лейбл `projects.deckhouse.io/skip-heritage-label`.

Пример:

{% raw %}

```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: {{ .projectName }}
  labels:
    projects.deckhouse.io/skip-heritage-label: "true"
    app: my-app
data:
  key: value
```

{% endraw %}

В этом случае ресурс получит лейблы `projects.deckhouse.io/project` и `projects.deckhouse.io/project-template`, но не получит лейбл `heritage: multitenancy-manager`.

### Исключение ресурсов из управления multitenancy-manager

Если необходимо исключить ресурс из управления `multitenancy-manager` (например, если он должен управляться вручную или другим контроллером), добавьте к ресурсу лейбл `projects.deckhouse.io/unmanaged`.

Пример:

{% raw %}

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: external-secret
  namespace: {{ .projectName }}
  labels:
    projects.deckhouse.io/unmanaged: "true"
type: Opaque
data:
  token: <base64-encoded-value>
```

{% endraw %}

Ресурсы с лейблом `projects.deckhouse.io/unmanaged`:

- Будут созданы **только один раз** при создании проекта;
- **Не будут обновляться** при последующих изменениях шаблона или обновлениях;
- Не будут отслеживаться в статусе проекта;
- Получат лейблы `projects.deckhouse.io/project` и `projects.deckhouse.io/project-template`, но **не получат** лейбл `heritage: multitenancy-manager`.

{% alert level="warning" %}
После того как ресурс помечен как `unmanaged`, он будет создан при первой установке, но не будет обновляться при изменении ProjectTemplate.
После создания ресурс становится полностью независимым и должен управляться вручную.
{% endalert %}

## Реализация валидации изменений объектов с помощью пользовательского лейбла

Модуль `multitenancy-manager` использует `ValidatingAdmissionPolicy` для защиты ресурсов с лейблом `heritage: multitenancy-manager` от ручных изменений.  
Вы можете реализовать аналогичную валидацию для ресурсов с любым лейблом.

### Как работает валидация в multitenancy-manager

Происходит валидация объектов с лейблом `heritage: multitenancy-manager`.  
Для этого используются следующие ресурсы:

1. ValidatingAdmissionPolicy — определяет правила валидации:
   - Операции: `UPDATE` и `DELETE`;
   - Проверка: разрешены только операции от имени service account контроллера;
   - Применяется ко всем ресурсам и API группам.

1. ValidatingAdmissionPolicyBinding — определяет на какие объекты распространяется валидация:
   - Использует `namespaceSelector` и `objectSelector` для выбора ресурсов по лейблу `heritage: multitenancy-manager`.

### Создание собственной валидации

Для реализации валидации для ресурсов с другим лейблом (например, `heritage: my-custom-label`):

1. Создайте файл с манифестами ресурсов ValidatingAdmissionPolicy и ValidatingAdmissionPolicyBinding:

   ```yaml
   apiVersion: admissionregistration.k8s.io/v1
   kind: ValidatingAdmissionPolicy
   metadata:
     name: my-custom-label-validation
   spec:
     failurePolicy: Fail
     matchConstraints:
       resourceRules:
         - apiGroups:   ["*"]
           apiVersions: ["*"]
           operations:  ["UPDATE", "DELETE"]
           resources:   ["*"]
           scope: "*"
     validations:
       - expression: 'request.userInfo.username == "system:serviceaccount:my-namespace:my-service-account"' # Замените на ваш service account.
         reason: Forbidden
         messageExpression: 'object.kind == ''Namespace'' ? ''This resource is managed by '' + object.metadata.name + '' system. Manual modification is forbidden.''
           : ''This resource is managed by '' + object.metadata.namespace + '' system. Manual modification is forbidden.'''
   ---
   apiVersion: admissionregistration.k8s.io/v1
   kind: ValidatingAdmissionPolicyBinding
   metadata:
     name: my-custom-label-validation
   spec:
     policyName: my-custom-label-validation
     validationActions: [Deny, Audit]
     matchResources:
       namespaceSelector:
         matchLabels:
           heritage: my-custom-label
       objectSelector:
         matchLabels:
           heritage: my-custom-label
   ```

1. Настройте параметры валидации:

   - `policyName` — уникальное имя политики (должно совпадать с `Policy` и `Binding`);
   - `request.userInfo.username` — имя service account, которому разрешено изменять ресурсы (замените на ваш service account);
   - `heritage: my-custom-label` — значение лейбла `heritage` для ваших ресурсов (замените на ваше значение). Запрещено использование значение `multitenancy-manager`, `deckhouse`;
   - `failurePolicy: Fail` — политика при ошибке валидации:
     - `Fail` — отклонять запрос при ошибке проверки,
     - `Ignore` — игнорировать ошибки валидации.
   - `validationActions` — действия валидации:
     - `Deny` — отклонять неразрешенные операции,
     - `Audit` — записывать операции в аудит лог.

1. Примените политику:

   ```shell
   d8 k apply -f my-validation-policy.yaml
   ```

1. Убедитесь, что ваши ресурсы имеют соответствующий лейбл `heritage`:

   ```yaml
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: my-resource
     labels:
       heritage: my-custom-label
   ```

## Выдача кластерных ресурсов проектам

`multitenancy-manager` позволяет администраторам кластера управлять тем, какие кластерные ресурсы (например, StorageClass) можно использовать из неймспейсов проектов.

Для этого используются кастомные ресурсы:

- `GrantableClusterResourceDefinition` (cluster-scoped) — регистрирует кластерный ресурс, который
  можно выдавать проектам: какой это ресурс (`grantedResource`), где проверяются ссылки на него
  (`usageReferences`), базовая доступность (`defaultAvailability`) и как определяется дефолт проекта
  (`defaultFrom`). Каждая ссылка отдельно включает подстановку дефолта через `default: true` —
  ставьте его только для поля, значение которого ресурсу всегда нужно (например, `storageClassName`
  у `PersistentVolumeClaim`). Для ссылки, отсутствие которой осмысленно (например, аннотация-
  переключатель функции), не ставьте: такая ссылка по-прежнему проверяется и учитывается в квоте,
  но никогда не заполняется.
- `ClusterResourceGrantPolicy` (cluster-scoped) — выбирает проекты (по меткам неймспейса через
  `projectSelector`) и для каждого ресурса (`resourceName`) задаёт разрешённые имена (`allowed`,
  `allowedSelector`) и `default`. Allow-лист ограничивает ресурс этим списком.
- `AvailableClusterResource` (namespaced, read-only, короткое имя `available`) — формируемый контроллером каталог доступных для проекта кластерных ресурсов. Пользователи проекта читают его, чтобы узнать
  доступные имена. Изменять и удалять объекты каталога вручную нельзя.

{% raw %}

```yaml
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: storageclasses
spec:
  grantedResource:
    apiVersion: storage.k8s.io/v1
    kind: StorageClass
  enforcement: Managed
  defaultAvailability: All
  defaultFrom:
    annotationKey: storageclass.kubernetes.io/is-default-class
  usageReferences:
    - rule:
        apiGroups:
          - ""
        apiVersions:
          - v1
        resources:
          - persistentvolumeclaims
      fieldPath: $.spec.storageClassName
      default: true
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: production-storage
spec:
  projectSelector:
    matchLabels:
      environment: production
  resources:
    - resourceName: storageclasses
      default: fast-ssd          # Перекрывает дефолт по аннотации.
      allowed:
        - fast-ssd
        - standard
      allowedSelector:           # Плюс любой StorageClass с меткой shared=true.
        matchLabels:
          shared: "true"
```

{% endraw %}

Особенности применения:

- Проверяющий (validating) вебхук запрещает создание/обновление объектов в подходящих проектах, если
  используемое значение не разрешено. Уже присутствующие в объекте значения при обновлении не блокируются — существующие объекты продолжают работать.
- Мутирующий (mutating) вебхук подставляет значение по умолчанию только при создании и только в
  ссылки, помеченные `default: true`. Ссылки без неё (например, аннотации-переключатели) никогда
  не заполняются.
- Grant без совпавших проектов (или проект без совпавших grant’ов) ничего не ограничивает.

### Выдача кластерных ресурсов через шаблон проекта

Правила доступности кластерных ресурсов можно задавать прямо в [структурированном шаблоне](#структурированные-шаблоны) — тогда они автоматически применяются ко всем проектам, созданным из этого шаблона:

- `spec.resources` — правила «внутри» шаблона: тот же формат, что и `resources` в `ClusterResourceGrantPolicy` (имя ресурса, `allowed`/`allowedSelector`, `default`);
- `spec.grantPolicies` — список имён **библиотечных** политик `ClusterResourceGrantPolicy`. Библиотечная политика описывает переиспользуемый набор правил и не должна иметь `projectSelector` — к каким проектам её применять, определяет ссылающийся шаблон. Так, например, политику «корпоративные StorageClass» может поддерживать один администратор, а использовать — несколько шаблонов.

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ProjectTemplate
metadata:
  name: my-template
spec:
  resources:
    - resourceName: storageclasses
      allowed: ["standard"]
      default: standard
  grantPolicies:
    - corporate-issuers   # Библиотечная ClusterResourceGrantPolicy без projectSelector.
```

Для каждого источника контроллер создаёт служебную политику с именем `template-<шаблон>-<источник>` (для `spec.resources` — `template-<шаблон>-inline`); имя `inline` для библиотечной политики зарезервировано. Ссылка на несуществующую политику или на политику с `projectSelector` отклоняется при создании шаблона.
