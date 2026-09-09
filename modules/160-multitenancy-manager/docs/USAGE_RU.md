---
title: "Модуль multitenancy-manager: примеры использования"
---
{% raw %}

## Шаблоны для проектов доступные по умолчанию

В Deckhouse Kubernetes Platform есть набор шаблонов для создания проектов. Они идут по нарастающей: каждый следующий включает возможности предыдущего и добавляет свои. Значения параметров задаются в поле `.spec.parameters` ресурса Project:

- `simple` — минимальный шаблон, создающий только неймспейс проекта. Используйте его, когда нужно лишь изолированный неймспейс, управляемый как проект, а доступ и ограничения настраиваются через [стандартные поля](#стандартные-поля-проекта) и [привязки ролей проекта](#предоставление-доступа-внутри-проекта).

  Параметры:
  - `namespace.labels` и `namespace.annotations` — дополнительные лейблы и аннотации неймспейса проекта.

- `default` — шаблон для базовых сценариев использования проектов. В дополнение к неймспейсу он настраивает сетевую изоляцию, профиль безопасности подов, расширенный мониторинг и доставку логов.

  Параметры (кроме перечисленных для `simple`):
  - `networkPolicy` — `Isolated` (по умолчанию) запрещает весь трафик, кроме трафика внутри неймспейсов проекта, DNS, сбора метрик Prometheus и ingress-nginx; `NotRestricted` разрешает весь трафик.
  - `podSecurityProfile` — профиль [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) для неймспейсов проекта: `Baseline` (по умолчанию) запрещает известные способы повышения привилегий, `Restricted` применяет максимально строгие практики, `Privileged` не ограничивает ничего.
  - `extendedMonitoringEnabled` (по умолчанию `true`) — алерты о недоступности и перезапусках контроллеров, ошибках 5xx в ingress-nginx и нехватке свободного места на persistent volume'ах проекта.
  - `clusterLogDestinationName` — имя ресурса ClusterLogDestination, в который отправлять логи проекта. Если не задан, логи проекта никуда не отправляются.

- `secure` — включает все возможности шаблона `default`, а также ограничение пользователей и групп внутри контейнеров, аудит их обращений к ядру и сканирование образов на уязвимости.

  Параметры (кроме перечисленных для `default`):
  - `allowedUIDs` и `allowedGIDs` — диапазоны (`min`, `max`) идентификаторов, допустимых для пользователей и групп внутри контейнеров проекта. См. [Security Context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod).
  - `runtimeAuditEnabled` (по умолчанию `false`) — правила аудита обращений к ядру, выявляющие вредоносную активность. Работают, только если задан диапазон UID/GID.
  - `securityScanningEnabled` (по умолчанию `true`) — периодическое сканирование запускаемых образов на известные уязвимости (CVE) средствами Trivy, раз в 24 часа.

- `secure-with-dedicated-nodes` — включает все возможности шаблона `secure`, а также размещение проекта на выделенных узлах.

  Параметры (кроме перечисленных для `secure`), задать нужно хотя бы один из двух:
  - `dedicatedNodes.nodeSelector` — селектор узлов проекта. Селектор узла у создаваемого пода **заменяется** на это значение.
  - `dedicatedNodes.defaultTolerations` — tolerations в формате `spec.tolerations` пода. **Добавляются** к создаваемым подам проекта.

Шаблоны `default`, `secure` и `secure-with-dedicated-nodes` описаны в [структурированном виде](#структурированные-шаблоны) (`deckhouse.io/v1alpha2`); шаблон `simple` — минимальный устаревший (`v1alpha1`) шаблон.

Точный набор параметров смотрите в шаблоне, установленном в вашем кластере, — он соответствует версии платформы:

Для просмотра схемы параметров шаблона используйте команду:

```shell
d8 k get projecttemplates <ИМЯ_ШАБЛОНА_ПРОЕКТА> -o jsonpath='{.spec.parametersSchema.openAPIV3Schema}' | jq
```

Для просмотра шаблона целиком используйте команду:

```shell
d8 k get projecttemplates <ИМЯ_ШАБЛОНА_ПРОЕКТА> -o yaml
```

## Создание проекта

Чтобы создать проект, выполните следующие шаги:

1. Создайте ресурс [Project](cr.html#project) с указанием имени шаблона проекта в поле [.spec.projectTemplateName](cr.html#project-v1alpha3-spec-projecttemplatename).
1. Задайте [стандартные поля](#стандартные-поля-проекта) — [.spec.administrators](cr.html#project-v1alpha3-spec-administrators) и [.spec.quota](cr.html#project-v1alpha3-spec-quota), — которые теперь управляются непосредственно ресурсом Project независимо от шаблона.
1. В параметре [.spec.parameters](cr.html#project-v1alpha3-spec-parameters) ресурса Project укажите значения параметров для секции [.spec.parametersSchema.openAPIV3Schema](cr.html#projecttemplate-v1alpha2-spec-parametersschema-openapiv3schema) ресурса ProjectTemplate.

   Пример создания проекта с помощью ресурса [Project](cr.html#project) из шаблона `default` [ProjectTemplate](cr.html#projecttemplate) представлен ниже:

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

### Создание проекта без шаблона

Поле `projectTemplateName` необязательно. Проект без шаблона состоит только из неймспейса и [стандартных полей](#стандартные-поля-проекта) (администраторы, квота) — никакие политики из шаблонов в нём не создаются. Это удобно, когда настройки не нужны или управляются другими средствами:

```yaml
apiVersion: deckhouse.io/v1alpha3
kind: Project
metadata:
  name: my-plain-project
spec:
  administrators:
    - kind: Group
      name: k8s-admins
  quota:
    requests.cpu: "2"
```

Шаблон можно назначить позже, указав его в `.spec.projectTemplateName`.

### Правила именования проектов

Имя проекта одновременно является именем его основного неймспейса, поэтому при создании проекта проверяются следующие правила:

- имя не может начинаться с `d8-` и `kube-` — эти префиксы зарезервированы за системными неймспейсами;
- имя не может быть длиннее 61 символа;
- если существует проект `foo`, нельзя создать проект `foo-bar` — и наоборот, при существующем проекте `foo-bar` нельзя создать проект `foo`. Имена вида `<проект>-*` зарезервированы под [дополнительные неймспейсы](#дополнительные-неймспейсы-проекта) проекта: без этого правила дополнительный неймспейс одного проекта мог бы совпасть по имени с чужим проектом.

## Статус и диагностика проекта

Поле `.status.state` проекта принимает значение `Deployed` (все ресурсы проекта синхронизированы) либо `Error`. Причина ошибки описывается в условиях (`.status.conditions`):

```shell
d8 k get project my-project -o jsonpath='{range .status.conditions[*]}{.type}={.status}: {.message}{"\n"}{end}'
```

| Условие | Значение `False` означает |
|---------|---------------------------|
| `ProjectTemplateFound` | Шаблон, указанный в `.spec.projectTemplateName`, не найден. |
| `Validated` | Параметры проекта не прошли валидацию по схеме шаблона (`parametersSchema`). |
| `ResourcesUpgraded` | Не удалось создать или обновить ресурсы проекта из шаблона (детали — в `message`). |
| `StandardFieldsApplied` | Не удалось применить [стандартные поля](#стандартные-поля-проекта) (квоту или администраторов). |
| `TemplateRolesAllowed` | Шаблон создаёт привязку к роли, [запрещённой для выдачи в проектах](#предоставление-доступа-внутри-проекта), — проект переводится в `Error`, в `message` указана роль.|
| `TemplateResourcesFiltered` | Из шаблона были отброшены объекты ResourceQuota/AuthorizationRule (см. [стандартные поля](#стандартные-поля-проекта)). Условие информационное — проект продолжает работать. |

Прочие полезные поля статуса:

- `.status.namespaces` — все неймспейсы проекта с указанием их типа (`Main`/`Additional`);
- `.status.usage` — текущее потребление квоты (заполняется при заданном `.spec.quota`);
- `.status.resources` — состояние отдельных ресурсов, созданных из шаблона.

### Служебные объекты проекта

Контроллер создаёт в неймспейсах проекта служебные объекты. Они управляются автоматически — редактировать их вручную нельзя (попытка будет отклонена):

| Объект | Где | Откуда берётся |
|--------|-----|----------------|
| `ResourceQuota/d8-project-quota` | Основной неймспейс | Поле [`.spec.quota`](cr.html#project-v1alpha3-spec-quota) проекта. |
| `ProjectRoleBinding/d8-administrators` | Основной неймспейс | Поле [`.spec.administrators`](cr.html#project-v1alpha3-spec-administrators) проекта. |
| `RoleBinding/d8:prb:<имя>` | Каждый неймспейс проекта | Разворачивание [ProjectRoleBinding](cr.html#projectrolebinding) с именем `<имя>`. |
| `RoleBinding/d8:cprb:<имя>` | Каждый неймспейс каждого проекта | Разворачивание [ClusterProjectRoleBinding](cr.html#clusterprojectrolebinding) с именем `<имя>` |

При удалении исходного объекта (привязки, поля квоты и т. д.) соответствующие служебные объекты удаляются автоматически.

## Виртуальные проекты

Помимо созданных пользователями проектов, в списке `d8 k get projects` всегда присутствуют два **виртуальных** проекта (лейбл `projects.deckhouse.io/virtual-project: "true"`):

- `deckhouse` — объединяет системные неймспейсы (с префиксами `d8-` и `kube-`);
- `default` — объединяет все остальные неймспейсы, не принадлежащие ни одному проекту.

Виртуальные проекты нужны для полноты картины: с ними каждый неймспейс кластера относится к какому-то проекту. Управлять ими нельзя: они не редактируются, в них нельзя создавать [ProjectNamespace](cr.html#projectnamespace) и [ProjectRoleBinding](cr.html#projectrolebinding), и на них не распространяются [ClusterProjectRoleBinding](cr.html#clusterprojectrolebinding).

## Дополнительные неймспейсы проекта

Если приложению нужно несколько неймспейсов (например, отдельное для кеша или очередей), добавьте их в проект ресурсом [ProjectNamespace](cr.html#projectnamespace). Ресурс создаётся **в основном неймспейсе проекта**; итоговый неймспейс получает имя `<имя проекта>-<spec.name>`:

```yaml
apiVersion: deckhouse.io/v1alpha3
kind: ProjectNamespace
metadata:
  name: cache
  namespace: my-project
spec:
  name: cache   # Будет создан неймспейс my-project-cache.
```

Проверить состав проекта можно по его статусу:

```shell
d8 k get project my-project -o jsonpath='{.status.namespaces}'
```

Правила работы с ProjectNamespace:

- Поле `spec.name` неизменяемо: чтобы переименовать неймспейс, удалите ресурс и создайте новый.
- Итоговое имя `<имя проекта>-<spec.name>` не может быть длиннее 63 символов (ограничение Kubernetes на имена неймспейсов).
- Создавать ProjectNamespace можно только в основном неймспейсе проекта — «вложить» его в дополнительный неймспейс или чужой проект нельзя. Если неймспейс с таким именем уже существует и принадлежит другому проекту, запрос будет отклонён.
- При удалении ресурса ProjectNamespace удаляется его неймспейс. При удалении проекта удаляются все его неймспейсы.

### Что распространяется на дополнительные неймспейсы

Автоматически действует во **всех** неймспейсах проекта (и в основном, и в дополнительных):

- **Доступ**: привязки [ProjectRoleBinding](cr.html#projectrolebinding) и [ClusterProjectRoleBinding](cr.html#clusterprojectrolebinding), включая автоматический доступ администраторов проекта. При добавлении нового неймспейса все существующие привязки разворачиваются в него без каких-либо действий со стороны пользователя.
- **Namespaced-объекты шаблона**: сетевая политика (`networkPolicy.mode: Isolated`) и настройка сбора логов (`logShipping`) создаются в каждом неймспейсе проекта. Сетевая изоляция при этом разрешает трафик между неймспейсами одного проекта.
- **Кластерные политики шаблона** (`OperationPolicy`, `SecurityPolicy` из `allowedUIDs`/`allowedGIDs`): выбирают неймспейсы по лейблу `projects.deckhouse.io/project`, то есть покрывают весь проект.
- **Наследуемые лейблы**: профиль безопасности подов (`security.deckhouse.io/pod-policy`), расширенный мониторинг (`extended-monitoring.deckhouse.io/enabled`), сканирование уязвимостей (`security-scanning.deckhouse.io/enabled`) и лейбл шаблона (`projects.deckhouse.io/project-template`) синхронизируются с основного неймспейса на дополнительные. Синхронизация полная: если фичу выключили в шаблоне, лейбл снимется и с дополнительных неймспейсов. Благодаря лейблу шаблона [правила доступности кластерных ресурсов](#управление-доступом-к-cluster-wide-ресурсам) тоже действуют во всех неймспейсах проекта.

Действуют только в **основном** неймспейсе:

- квота проекта (`ResourceQuota` из [`.spec.quota`](cr.html#project-v1alpha3-spec-quota));
- дополнительные лейблы и аннотации из `namespaceMetadata` шаблона;
- аннотации размещения на узлах (из полей `nodeSelector` и `tolerations` шаблона).

### Лейблы неймспейсов проекта

| Лейбл | Основной | Дополнительные | Назначение |
|-------|:--------:|:--------------:|------------|
| `projects.deckhouse.io/project: <имя проекта>` | ✓ | ✓ | Принадлежность к проекту — общий лейбл всех неймспейсов проекта. |
| `projects.deckhouse.io/project-namespace: <spec.name>` | — | ✓ | Признак дополнительного неймспейса (имя ресурса ProjectNamespace). |
| `projects.deckhouse.io/project-template: <имя шаблона>` | ✓ | ✓ | Шаблон проекта; по нему применяются правила доступности кластерных ресурсов. |
| `heritage: multitenancy-manager` | ✓ | ✓ | Неймспейс управляется контроллером проектов; вручную его менять нельзя. |
| `security.deckhouse.io/pod-policy`, `extended-monitoring.deckhouse.io/enabled`, `security-scanning.deckhouse.io/enabled` | ✓ | ✓ (наследуются) | Политики и фичи из шаблона проекта. |

Общий лейбл `projects.deckhouse.io/project` позволяет выбирать неймспейсы проекта с помощью команды `get ns`. Примеры:

Получить все неймспейсы проекта (основное + дополнительные):

```shell
d8 k get ns -l projects.deckhouse.io/project=my-project
```

Получить только дополнительные неймспейсы проекта:

```shell
d8 k get ns -l 'projects.deckhouse.io/project=my-project,projects.deckhouse.io/project-namespace'
```

Получить только основной неймспейс проекта:

```shell
d8 k get ns -l 'projects.deckhouse.io/project=my-project,!projects.deckhouse.io/project-namespace'
```

## Автоматическое создание проекта для неймспейса

По умолчанию (параметр [`allowNamespacesWithoutProjects: true`](configuration.html#parameters-allownamespaceswithoutprojects)) неймспейс, созданный напрямую (например, `d8 k create ns my-app`), автоматически оборачивается в проект с тем же именем:

- проект создаётся без шаблона и помечается лейблом `multitenancy.deckhouse.io/project-managed-by-namespace: "true"`;
- источник истины — неймспейс: его лейблы и аннотации синхронизируются в параметры проекта; редактируйте и удаляйте именно неймспейс (при его удалении проект удаляется автоматически);
- редактировать спецификацию такого проекта вручную нельзя. Чтобы превратить его в обычный проект (например, назначить шаблон), снимите с проекта лейбл `multitenancy.deckhouse.io/project-managed-by-namespace` — после этого проект будет управляться как обычно.

Если параметр `allowNamespacesWithoutProjects` выключен, создание неймспейса вне проектов запрещено — попытка выполнить команду `d8 k create ns` будет отклонена с пояснением.

Существующий неймспейс также можно явно принять в управление проектом, пометив его аннотацией `projects.deckhouse.io/adopt`. Например:

1. Создайте новый неймспейс:

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

   В списке проектов появится новый проект, соответствующий неймспейсу:

   ```shell
   NAME        STATE      PROJECT TEMPLATE   DESCRIPTION                                            AGE
   deckhouse   Deployed   virtual            This is a virtual project                              181d
   default     Deployed   virtual            This is a virtual project                              181d
   test        Deployed                                                                             1m
   ```

Шаблон созданного проекта можно изменить на существующий.

{% endraw %}

{% alert level="warning" %}
Обратите внимание, что при смене шаблона может возникнуть конфликт ресурсов: если в чарте шаблона прописаны ресурсы, которые уже присутствуют в пространстве имён, то применить шаблон не получится.
{% endalert %}

{% raw %}

## Стандартные поля проекта

Администраторы проекта и квоты ресурсов больше не являются параметрами шаблона — это поля верхнего уровня ресурса [Project](cr.html#project), работающие с любым шаблоном (включая `simple` и проекты без шаблона):

- `.spec.administrators` — список субъектов (`kind: User` или `kind: Group` и `name`), получающих административный доступ к проекту. Контроллер реализует этот доступ через автоматически создаваемый [ProjectRoleBinding](cr.html#projectrolebinding) в неймспейсе проекта.
- `.spec.quota` — набор жёстких лимитов [ResourceQuota](https://kubernetes.io/docs/concepts/policy/resource-quotas/) (например, `requests.cpu`, `limits.memory`). Контроллер поддерживает `ResourceQuota` в неймспейсе проекта и сообщает текущее потребление в `.status.usage`. Для `memory` и `storage` должна быть указана единица измерения (например `2Gi`). Числа без единицы измерения означают байты и отклоняются.

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
Объекты ResourceQuota и AuthorizationRule, описанные внутри шаблонов проектов, больше не рендерятся: такие ресурсы теперь управляются исключительно через `.spec.quota` и `.spec.administrators`. Существующие шаблоны, в которых они объявлены, продолжают работать, но эти объекты отфильтровываются при рендеринге.
{% endalert %}

## Предоставление доступа внутри проекта

Чтобы предоставить доступ к неймспейсам проекта пользователям помимо администраторов проекта, используйте привязки ролей, которые ссылаются на кластерные роли и автоматически разворачиваются в нужные неймспейсы проектов:

- [ProjectRoleBinding](cr.html#projectrolebinding) (неймспейс, короткое имя `prb`) — предоставляет роль в рамках **одного** проекта. Должен создаваться в главном неймспейсе проекта (имя которого совпадает с именем проекта). Контроллер создаёт RoleBinding в каждом неймспейсе этого проекта.
- [ClusterProjectRoleBinding](cr.html#clusterprojectrolebinding) (кластерный, короткое имя `cprb`) — предоставляет роль во **всех** невиртуальных проектах. Контроллер создаёт RoleBinding в каждом неймспейсе каждого проекта и сообщает количество затронутых проектов в `.status.boundProjects`.

`roleRef` должен ссылаться на `ClusterRole`, имя которого начинается с одного из разрешённых префиксов (`d8:project:`, `d8:namespace:`, `d8:project-capability:`, `d8:namespace-capability:`, `d8:custom:`). Описание ролей — [в документации модуля user-authz](../user-authz/).

При создании привязок действуют следующие проверки:

- **Защита от повышения привилегий**: создать привязку может только пользователь, у которого есть право привязывать (`bind`) указанную роль. Например, администратор проекта (`d8:project:admin`) может выдавать встроенные роли `d8:project:*` и `d8:namespace:*`, но не может выдать роль шире своих полномочий.
- Роль должна существовать: привязка к несуществующей роли отклоняется.
- ServiceAccount в качестве субъекта ProjectRoleBinding должен принадлежать неймспейсу этого же проекта.
- Системные и подсистемные роли (`d8:system:*`, `d8:subsystem:*`), а также произвольные роли вне перечисленных префиксов через проектные привязки выдать нельзя.
- Роли с аннотацией `rbac.deckhouse.io/disabled-for-direct-use-in-projects: "true"` запрещены для выдачи в проектах. Эту аннотацию администратор кластера может поставить на роль, чтобы прекратить ее использование: при этом существующие привязки продолжают работать, но новые не создаются. Если такую роль использует шаблон проекта, проект переходит в статус `Error` с пояснением в условии `TemplateRolesAllowed`.

Привязка `d8-administrators`, создаваемая контроллером из поля [`.spec.administrators`](cr.html#project-v1alpha3-spec-administrators), управляется только контроллером — редактировать её вручную нельзя. Чтобы изменить состав администраторов, измените поле `.spec.administrators` проекта.

### Роли, доступные в RoleBinding внутри проекта

Кроме проектных привязок, внутри неймспейса проекта можно использовать и обычный RoleBinding — тогда роль действует только в этом одном неймспейсе. Но в проектах набор ролей, доступных для обычного RoleBinding, ограничен: разрешены только кластерные роли с лейблом `rbac.deckhouse.io/delegatable: "true"`. Из встроенных это роли `d8:namespace:*` и `d8:project:*`, а также роли уровней доступа упрощённой ролевой модели (`user-authz:user`, `user-authz:privileged-user`, `user-authz:editor`, `user-authz:admin`).

RoleBinding на любую другую кластерную роль (например, `cluster-admin`, системные роли или capabilities) в проекте будет отклонён с сообщением `references "<роль>" which is not available to project`. Это защита от обхода изоляции проекта через привязку к слишком широкой роли.

Чтобы использовать в проектах [собственную роль](/modules/user-authz/faq.html#создание-собственной-namespace--или-проектной-роли), добавьте на неё лейбл `rbac.deckhouse.io/delegatable: "true"`:

```shell
d8 k label clusterrole d8:custom:namespace:developer rbac.deckhouse.io/delegatable=true
```

Ограничение действует только в неймспейсах «настоящих» проектов. На [автоматически обёрнутые](#автоматическое-создание-проекта-для-неймспейса) неймспейсы (с лейблом `multitenancy.deckhouse.io/project-managed-by-namespace`) оно не распространяется.

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

Начиная с API-версии `deckhouse.io/v1alpha2`, шаблон проекта описывается **структурированными полями** — вместо текстового Helm-шаблона вы декларативно указываете, какие настройки получат неймспейсы проекта. Контроллер сам создаёт из этих полей нужные объекты (сетевые политики, политики безопасности, настройки сбора логов и т. д.) в каждом неймспейсе проекта и поддерживает их в актуальном состоянии.

Доступные поля (все — необязательные; полный справочник — [в описании ресурса ProjectTemplate](cr.html#projecttemplate)):

| Поле | Что настраивает |
|------|-----------------|
| `podSecurityStandard` | Профиль безопасности подов: `Privileged`, `Baseline` или `Restricted`. |
| `networkPolicy.mode` | Сетевая изоляция: `Isolated` (трафик разрешён только внутри проекта и от системных компонентов платформы) или `NotRestricted`. |
| `features.monitoring` | Расширенный мониторинг неймспейсов проекта. |
| `features.vulnerabilityScanning` | Сканирование образов контейнеров на уязвимости. |
| `logShipping.clusterDestinationRef` | Сбор логов подов проекта в указанное хранилище (`ClusterLogDestination`). |
| `nodeSelector`, `tolerations` | Размещение подов проекта на выделенных узлах. |
| `allowedUIDs`, `allowedGIDs` | Допустимые диапазоны UID/GID контейнеров проекта. |
| `runtimeAudit.enabled` | Аудит обращений процессов проекта к ядру Linux .|
| `namespaceMetadata.labels`, `namespaceMetadata.annotations` | Дополнительные лейблы и аннотации неймспейсов проекта. |
| `resources`, `grantPolicies` | [Выдача кластерных ресурсов через шаблон проекта](#выдача-кластерных-ресурсов-через-шаблон-проекта). |
| `parametersSchema.openAPIV3Schema` | Схема параметров, которые задаются при создании проекта. |

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

Любое поле шаблона, которое хранит значение, а не вложенную структуру, можно сделать параметром: вместо конкретного значения укажите `{fromParam: <имя параметра>}` и объявите параметр в `parametersSchema`. Значение не обязательно должно быть скалярным: параметром может быть и словарь (`nodeSelector`, `labels`), и список (`tolerations`), и объект (`allowedUIDs`). Тогда каждый проект задаёт своё значение в `.spec.parameters`, а если значение не задано — используется `default` из схемы.

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

Для действий над шаблонами существуют следующие правила:

- Шаблон, который используется хотя бы одним проектом, нельзя удалить.
- Изменение шаблона автоматически применяется ко всем проектам, созданным из него.
- Устаревшие шаблоны `deckhouse.io/v1alpha1` с текстовым полем `resourcesTemplate` (Helm-шаблонизация) продолжают работать, но признаны устаревшими — новые шаблоны создавайте в структурированном виде. Ресурсы ResourceQuota и AuthorizationRule из таких шаблонов отфильтровываются при рендеринге (подробнее — в разделе [«Стандартные поля проекта»](#стандартные-поля-проекта)).

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
Он запрещает изменение ресурсов пользователями или любым контроллером, кроме `multitenancy-manager`.  
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

## Управление доступом к cluster-wide-ресурсам

Модуль позволяет управлять доступом проектов к cluster-wide-ресурсам,
таким как StorageClass, ClusterIssuer, ClusterRole и другим.

Описание механизма, используемых ресурсов и cluster-wide-ресурсов, зарегистрированных платформой,
приведено [на странице описания модуля](./#управление-доступом-к-cluster-wide-ресурсам).

Далее приведены основные сценарии настройки и использования механизма.

### Для администраторов кластера

Ниже приведены примеры настройки доступа проектов к cluster-wide-ресурсам с помощью ClusterResourceGrantPolicy.

#### Ограничение StorageClass для проекта

Чтобы разрешить проектам использовать только StorageClass с именами `fast-ssd` и `standard`, а `fast-ssd` использовать по умолчанию, создайте следующий [ресурс ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy):

{% raw %}

```yaml
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
      default: fast-ssd
      allowed:
        - fast-ssd
        - standard
```

{% endraw %}

При создании PersistentVolumeClaim без значения `spec.storageClassName` в это поле автоматически подставляется `fast-ssd`. Если указан StorageClass, которого нет в списке разрешённых, создание PersistentVolumeClaim отклоняется.

Для StorageClass используется режим подстановки значения по умолчанию [`Coerce`](cr.html#grantableclusterresourcereference-v1alpha1-spec-fieldpaths-defaulting). Если встроенный admission-контроллер Kubernetes уже подставил в `spec.storageClassName` класс по умолчанию, недоступный проекту, значение заменяется на `fast-ssd`, а создание PersistentVolumeClaim не отклоняется.

Чтобы проверить, какие StorageClass доступны проекту, выполните следующую команду:

```shell
d8 k get available storageclasses -n <PROJECT_NAME> -o yaml
```

#### Ограничение ClusterIssuer для проекта

Чтобы разрешить проектам использовать только ClusterIssuer с именами `letsencrypt-prod` и `vault-issuer`, а `letsencrypt-prod` использовать по умолчанию, создайте следующий [ресурс ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy):

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: production-issuers
spec:
  projectSelector:
    matchLabels:
      environment: production
  resources:
    - resourceName: clusterissuers
      default: letsencrypt-prod
      allowed:
        - letsencrypt-prod
        - vault-issuer
```

{% endraw %}

Политика применяется к ClusterIssuer, указанному одним из следующих способов:

- в поле `.spec.issuerRef.name` ресурса Certificate, если в `.spec.issuerRef.kind` указано ClusterIssuer;
- в аннотации `cert-manager.io/cluster-issuer` ресурса Ingress.

При создании Certificate с ClusterIssuer без значения `.spec.issuerRef.name` в это поле автоматически подставляется `letsencrypt-prod`. Если указан ClusterIssuer, которого нет в списке разрешённых, создание Certificate отклоняется.

Для аннотации `cert-manager.io/cluster-issuer` значение по умолчанию не подставляется. Если аннотация указана, её значение проверяется по тому же списку разрешённых ClusterIssuer.

{% alert level="info" %}
Управление доступом к ClusterIssuer доступно только при включённом [модуле `cert-manager`](/modules/cert-manager/).
{% endalert %}

#### Предоставление доступа к дополнительным ClusterRole

По умолчанию в RoleBinding можно использовать только ClusterRole с лейблом `rbac.deckhouse.io/delegatable`.

Чтобы разрешить проектам команды `payments` использовать дополнительные ClusterRole, создайте следующий [ресурс ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy):

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: extra-roles
spec:
  projectSelector:
    matchLabels:
      team: payments
  resources:
    - resourceName: clusterroles
      allowed:
        - my-custom-role
      allowedSelector:
        matchLabels:
          shared: "true"
```

{% endraw %}

Политика дополнительно разрешает использовать:

- ClusterRole с именем `my-custom-role`;
- ClusterRole, соответствующие селектору `shared: "true"`.

ClusterRole с лейблом `rbac.deckhouse.io/delegatable` при этом остаются доступными.

При создании или изменении RoleBinding указанная в нём ClusterRole проверяется на доступность проекту. Значение ClusterRole автоматически не подставляется.

#### Ограничение LoadBalancerClass для сервиса

Чтобы разрешить проектам использовать только LoadBalancerClass со значениями `internal-lb` и `edge-lb`, а `internal-lb` использовать по умолчанию, создайте следующий [ресурс ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy):

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: lb-classes
spec:
  projectSelector:
    matchLabels:
      environment: staging
  resources:
    - resourceName: loadbalancerclasses
      default: internal-lb
      allowed:
        - internal-lb
        - edge-lb
```

{% endraw %}

В отличие от StorageClass, ClusterIssuer и ClusterRole, ресурс LoadBalancerClass не является отдельным ресурсом Kubernetes. Политика определяет допустимые значения поля `.spec.loadBalancerClass` для сервисов типа `LoadBalancer`.

При создании Service типа `LoadBalancer` без значения `.spec.loadBalancerClass` в это поле автоматически подставляется `internal-lb`. Если указано значение, которого нет в списке разрешённых, создание Service отклоняется.

На Service других типов политика не распространяется.

#### Предоставление доступа ко всем ресурсам определённого типа

Чтобы разрешить определённым проектам использовать все ресурсы выбранного типа без явного перечисления, установите [`availabilityDefault: All`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-availabilitydefault).

Следующая политика разрешает всем проектам с лейблом `environment: sandbox` использовать любые StorageClass:

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: open-storage-for-sandbox
spec:
  projectSelector:
    matchLabels:
      environment: sandbox
  resources:
    - resourceName: storageclasses
      availabilityDefault: All
```

{% endraw %}

Обычно для управления доступом достаточно явно указывать разрешённые ресурсы с помощью [`allowed`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-allowed) или [`allowedSelector`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-allowedselector). Используйте параметр `availabilityDefault: All`, если выбранным проектам необходимо предоставить доступ ко всем ресурсам указанного типа.

#### Запрет отдельных ресурсов

Чтобы запретить проектам использовать отдельные ресурсы, оставив остальные доступными, используйте параметр [`denied`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-denied) или [`deniedSelector`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-deniedselector).

Следующая политика запрещает проектам с лейблом `environment: dev` использовать StorageClass с именами `expensive-nvme` и `archived-hdd`:

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: deny-expensive-storage
spec:
  projectSelector:
    matchLabels:
      environment: dev
  resources:
    - resourceName: storageclasses
      denied:
        - expensive-nvme
        - archived-hdd
```

{% endraw %}

Запрет имеет приоритет над разрешением. Если ресурс соответствует одновременно `denied` или `deniedSelector` и `allowed` или `allowedSelector`, он считается недоступным.

#### Управление доступом с помощью label-селекторов

Чтобы управлять доступом к ресурсам без перечисления их имён, используйте параметры [`allowedSelector`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-allowedselector) и [`deniedSelector`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-deniedselector). Селекторы позволяют разрешать или запрещать ресурсы на основе их лейблов.

Следующая политика разрешает проектам с лейблом `tier: shared` использовать StorageClass с лейблом `shared: "true"`, за исключением StorageClass с лейблом `deprecated: "true"`:

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: shared-storage-only
spec:
  projectSelector:
    matchLabels:
      tier: shared
  resources:
    - resourceName: storageclasses
      allowedSelector:
        matchLabels:
          shared: "true"
      deniedSelector:
        matchLabels:
          deprecated: "true"
```

{% endraw %}

#### Выдача кластерных ресурсов через шаблон проекта

Правила доступности кластерных ресурсов можно задавать прямо в [структурированном шаблоне](#структурированные-шаблоны) — тогда они автоматически применяются ко всем проектам, созданным из этого шаблона:

- `spec.resources` — правила «внутри» шаблона: тот же формат, что и `resources` в ClusterResourceGrantPolicy (имя ресурса, `allowed`/`allowedSelector`, `default`);
- `spec.grantPolicies` — список имён **библиотечных** политик ClusterResourceGrantPolicy. Библиотечная политика описывает переиспользуемый набор правил и не должна иметь `projectSelector` — к каким проектам её применять, определяет ссылающийся шаблон. Так, например, политику «корпоративные StorageClass» может поддерживать один администратор, а использовать — несколько шаблонов.

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

### Для пользователей проекта

Пользователи проекта могут просматривать доступные им cluster-wide-ресурсы, а также проверять значения, используемые по умолчанию.

#### Просмотр доступных cluster-wide-ресурсов

В неймспейсе каждого проекта автоматически создаётся [ресурс AvailableClusterResource](cr.html#availableclusterresource) для каждого зарегистрированного
ресурса. С их помощью можно узнать, какие cluster-wide-ресурсы доступны проекту и какой ресурс используется по умолчанию.

Чтобы посмотреть все доступные cluster-wide-ресурсы проекта, выполните следующую команду:

```shell
d8 k get available -n <PROJECT_NAME>
```

Пример вывода:

```text
NAME                KIND         DEFAULT      AVAILABLE   AGE
storageclasses      StorageClass fast-ssd     2           5m
clusterissuers      ClusterIssuer letsencrypt 2           5m
```

Чтобы просмотреть подробную информацию о ресурсах определённого типа (например, StorageClass), выполните следующую команду:

```shell
d8 k get available storageclasses -n <PROJECT_NAME> -o yaml
```

#### Отказ в использовании cluster-wide-ресурса

Если при создании или изменении объекта указанный cluster-wide-ресурс недоступен проекту, операция отклоняется с сообщением:

```text
resource <RESOURCE_NAME> is not available to project <PROJECT_NAME>
```

В этом случае проверьте доступные ресурсы с помощью AvailableClusterResource. Используйте ресурс из списка доступных или попросите администратора кластера добавить необходимый ресурс.

#### Автоматическая подстановка значений по умолчанию

Для некоторых кластерных ресурсов администратор может задать значение по умолчанию. Если при создании объекта соответствующее значение не указано, оно подставляется автоматически.

Например, если для StorageClass по умолчанию задан `fast-ssd`, при создании PersistentVolumeClaim без `.spec.storageClassName` в это поле может быть автоматически подставлено значение `fast-ssd`.

Значение можно указать явно, выбрав любой доступный проекту ресурс из соответствующего AvailableClusterResource.

#### Случаи, когда у проекта может не быть дефолта

Дефолт проекта показывается в `status.default` ресурса AvailableClusterResource, а среди имён в
`status.available` он помечен флагом `default: true`. И того, и другого может не быть — это штатное
состояние, а не ошибка: ресурс может быть выдан проекту без имени, к которому следует откатываться.
Так бывает в четырёх случаях:

- в политике не задан `default`, а в регистрации нет `defaultFrom`;
- ресурс value-backed (выдаётся списком значений, а не объектами кластера) — `defaultFrom` к нему не
  применяется;
- `defaultFrom` задан, но объектов с этой аннотацией не ровно один: дефолт по аннотации определён,
  только когда на него претендует единственный объект;
- дефолт задан, но проекту недоступен — например, кластерный StorageClass по умолчанию не входит в
  выданные проекту имена. Он намеренно сбрасывается, чтобы мутирующий вебхук не подставил значение,
  которое тут же отвергнет проверяющий.

Практический смысл последнего: поле остаётся пустым, и объект создаётся (или отклоняется) по обычным
правилам — вместо отказа с упоминанием имени, которого пользователь проекта не выбирал.

### Для разработчиков модулей

#### Настройка проверки ссылки на cluster-wide-ресурс

Если ресурс модуля содержит поле со ссылкой на cluster-wide-ресурс, уже зарегистрированный с помощью [GrantableClusterResourceDefinition](cr.html#grantableclusterresourcedefinition), создайте [GrantableClusterResourceReference](cr.html#grantableclusterresourcereference). Он определяет, в каких ресурсах и полях используется cluster-wide-ресурс, а также позволяет настроить проверку доступности и автоматическую подстановку значения по умолчанию.

Например, чтобы настроить проверку StorageClass, указанного в поле `.spec.storageClassName` ресурса PostgresDatabase, добавьте в Helm-чарт модуля следующий ресурс:

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: postgresdatabases-storageclasses
  labels:
    heritage: deckhouse
    module: postgres
spec:
  grantableClusterResourceName: storageclasses
  rule:
    apiGroups: ["postgres.example.com"]
    apiVersions: ["*"]
    resources: ["postgresdatabases"]
  fieldPaths:
    - path: $.spec.storageClassName
      defaulting: Coerce
```

{% endraw %}

В этом примере `storageclasses` — имя существующего GrantableClusterResourceDefinition, а `Coerce` позволяет при создании PostgresDatabase подставить доступный проекту StorageClass по умолчанию, если значение отсутствует или недоступно проекту.

Описание параметров GrantableClusterResourceReference, режимов подстановки значений по умолчанию, условий `match` и настройки ресурсов с несколькими API-версиями приведено [в описании ресурса](cr.html#grantableclusterresourcereference).

#### Регистрация нового cluster-wide-ресурса

Чтобы добавить управление доступом к новому cluster-wide-ресурсу, выполните следующее:

1. Создайте ресурс [GrantableClusterResourceDefinition](cr.html#grantableclusterresourcedefinition), чтобы зарегистрировать cluster-wide-ресурс в механизме управления доступом.
1. Создайте один или несколько ресурсов [GrantableClusterResourceReference](cr.html#grantableclusterresourcereference), чтобы определить поля, в которых используются ссылки на него.

   Например, чтобы зарегистрировать ресурс MyClusterResource, добавьте в Helm-чарт модуля следующий ресурс:

   {% raw %}

   ```yaml
   apiVersion: multitenancy.deckhouse.io/v1alpha1
   kind: GrantableClusterResourceDefinition
   metadata:
     name: myclusterresources
     labels:
       heritage: deckhouse
       module: my-module
   spec:
     grantedResource:
       apiGroup: my.example.com
       kind: MyClusterResource
     enforcement: Managed
     defaultAvailability: All
     excluded:
       - matchExpressions:
           - key: my.example.com/internal
             operator: Exists
   ```

   {% endraw %}

   В этом примере зарегистрированные ресурсы по умолчанию доступны проектам. Ресурсы с меткой `my.example.com/internal` исключаются из доступных.

   Описание параметров ресурса GrantableClusterResourceDefinition и доступных режимов управления приведено [в описании ресурса](cr.html#grantableclusterresourcedefinition).

1. После регистрации cluster-wide-ресурса настройте ссылки на него с помощью GrantableClusterResourceReference, как описано [в подразделе «Настройка проверки ссылки на cluster-wide-ресурс»](#настройка-проверки-ссылки-на-cluster-wide-ресурс).

#### Использование x-deckhouse-grantable-resource в настройках приложений DKP

Для управления доступом к cluster-wide-ресурсам в настройках приложений DKP используйте OpenAPI-расширение `x-deckhouse-grantable-resource`. В этом случае deckhouse-контроллер автоматически проверяет доступность указанного ресурса и при необходимости подставляет значение по умолчанию. Создавать GrantableClusterResourceReference вручную не требуется.

Описание расширения и примеры использования приведены [в разделе «Разработка приложений»](/products/kubernetes-platform/documentation/v1/architecture/marketplace/application-development.html#подстановка-значения-из-грантов-на-ресурсы-кластера-x-deckhouse-grantable-resource).

#### Проверка состояния регистрации ресурса

Состояние GrantableClusterResourceDefinition и связанных с ним ресурсов GrantableClusterResourceReference можно проверить в поле `status`:

- [`GrantableClusterResourceDefinition.status.references`](cr.html#grantableclusterresourcedefinition-v1alpha1-status-references) — содержит список связанных ресурсов `GrantableClusterResourceReference` и информацию о ресурсах, к которым они применяются;
- [`GrantableClusterResourceReference.status.bound`](cr.html#grantableclusterresourcereference-v1alpha1-status-bound) — указывает, найден ли соответствующий GrantableClusterResourceDefinition;
- `GrantableClusterResourceReference.status.conditions[Bound]` — содержит состояние привязки: `Resolved`, если определение найдено, или `UnknownResource`, если оно отсутствует. Состояние `UnknownResource` может указывать на ошибку в имени GrantableClusterResourceDefinition или на отсутствие необходимой регистрации.
