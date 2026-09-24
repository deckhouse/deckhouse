---
title: Управление проектами
permalink: ru/admin/multitenancy/project-management.html
description: Управление проектами
lang: ru
---

В Deckhouse Platform есть набор шаблонов для создания проектов. Они кумулятивны: каждый следующий включает возможности предыдущего и добавляет свои. Значения параметров задаются в поле `.spec.parameters` ресурса Project:

- `simple` — минимальный шаблон, создающий только неймспейс проекта. Используйте его, когда нужно только изолированный неймспейс, управляемый как проект, а доступ и ограничения настраиваются через [стандартные поля](#стандартные-поля-проекта) и [привязки ролей проекта](#предоставление-доступа-внутри-проекта).

  Параметры:
  - `namespace.labels` и `namespace.annotations` — дополнительные лейблы и аннотации неймспейса проекта.
  - `requiredRequests` (по умолчанию `false`) — если `true`, у подов проекта должны быть заданы CPU и memory requests (OperationPolicy в режиме Deny). Шаблон создаёт только неймспейс, поэтому параметр по умолчанию выключен.

- `default` — шаблон для базовых сценариев использования проектов. В дополнение к неймспейсу он настраивает сетевую изоляцию, профиль безопасности подов, расширенный мониторинг и доставку логов.

  Параметры (кроме перечисленных для `simple`):
  - `networkPolicy` — `Isolated` (по умолчанию) запрещает весь трафик, кроме трафика внутри неймспейсов проекта, DNS, сбора метрик Prometheus и ingress-nginx; `NotRestricted` разрешает весь трафик.
  - `podSecurityProfile` — профиль [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) для неймспейсов проекта: `Baseline` (по умолчанию) запрещает известные способы повышения привилегий, `Restricted` применяет максимально строгие практики, `Privileged` не ограничивает ничего.
  - `extendedMonitoringEnabled` (по умолчанию `true`) — алерты о недоступности и перезапусках контроллеров, ошибках 5xx в ingress-nginx и нехватке свободного места на persistent volume'ах проекта.
  - `clusterLogDestinationName` — имя ресурса ClusterLogDestination, в который отправлять логи проекта. Если не задан, логи проекта никуда не отправляются.
  - `requiredRequests` (в этом шаблоне по умолчанию `true`) — если `true`, у подов проекта должны быть заданы CPU и memory requests (OperationPolicy в режиме Deny). При adopt существующего неймспейса параметр выставляется в `false`, чтобы не блокировать уже работающие нагрузки.

- `secure` — включает все возможности шаблона `default`, а также ограничение пользователей и групп внутри контейнеров, аудит их обращений к ядру и сканирование образов на уязвимости.

  Параметры (кроме перечисленных для `default`):
  - `allowedUIDs` и `allowedGIDs` — диапазоны (`min`, `max`) идентификаторов, допустимых для пользователей и групп внутри контейнеров проекта. См. [Security Context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod).
  - `runtimeAuditEnabled` (по умолчанию `false`) — правила аудита обращений к ядру, выявляющие вредоносную активность. Работают, только если задан диапазон UID/GID.
  - `securityScanningEnabled` (по умолчанию `true`) — периодическое сканирование запускаемых образов на известные уязвимости (CVE) средствами Trivy, раз в 24 часа.

- `secure-with-dedicated-nodes` — включает все возможности шаблона `secure`, а также размещение проекта на выделенных узлах.

  Параметры (кроме перечисленных для `secure`), задать нужно хотя бы один из двух:
  - `dedicatedNodes.nodeSelector` — селектор узлов проекта. Селектор узла у создаваемого пода **заменяется** на это значение.
  - `dedicatedNodes.defaultTolerations` — tolerations в формате `spec.tolerations` пода. **Добавляются** к создаваемым подам проекта.

Шаблоны `default`, `secure` и `secure-with-dedicated-nodes` описаны в [структурированном виде](#структурированные-шаблоны) (`deckhouse.io/v1alpha2`); шаблон `simple` — минимальный структурированный шаблон, который создаёт только неймспейс и задаёт его лейблы и аннотации из параметров проекта.

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

1. Создайте кастомный ресурс [Project](/modules/multitenancy-manager/cr.html#project) с указанием имени шаблона проекта в поле [.spec.projectTemplateName](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-projecttemplatename).
1. Задайте стандартные поля — [.spec.administrators](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-administrators) и [.spec.quota](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-quota), — которые управляются непосредственно ресурсом Project независимо от шаблона.
1. В параметре [.spec.parameters](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-parameters) укажите значения для секции [.spec.parametersSchema.openAPIV3Schema](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha2-spec-parametersschema-openapiv3schema) кастомного ресурса [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate).

   Пример создания проекта с помощью [Project](/modules/multitenancy-manager/cr.html#project) из `default` [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate) представлен ниже:

   ```yaml
   apiVersion: deckhouse.io/v1alpha3
   kind: Project
   metadata:
     name: my-project
   spec:
     description: This is an example from the Deckhouse documentation.
     projectTemplateName: default
     # Стандартные поля, управляемые самим ресурсом Project независимо от шаблона.
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

   {% alert level="info" %}
   API ресурса Project обслуживается как `deckhouse.io/v1alpha3`. Манифесты `v1alpha2` продолжают работать: вебхук конвертации автоматически переносит `parameters.administrators` и `parameters.resourceQuota` в стандартные поля `.spec.administrators` и `.spec.quota`. Версия `deckhouse.io/v1alpha1` больше не обслуживается.
   {% endalert %}

1. Для проверки статуса проекта выполните команду:

   ```shell
   d8 k get projects my-project
   ```

   Успешно созданный проект должен отображаться в статусе `Deployed` (синхронизирован). Если отображается статус `Error` (ошибка), добавьте аргумент `-o yaml` к команде (например, `d8 k get projects my-project -o yaml`) для получения более подробной информации о причине ошибки.

### Автоматическое создание проекта для неймспейса

Неймспейс, созданный напрямую (например, с использованием команды `d8 k create ns test`), автоматически становится проектом с тем же именем — никакая аннотация для этого не требуется:

- шаблон подбирается по тому, что на неймспейсе уже есть: `secure` — при лейбле `security-scanning.deckhouse.io/enabled`, `default` — при `security.deckhouse.io/pod-policy` или `extended-monitoring.deckhouse.io/enabled`, иначе `simple`;
- параметры проекта заполняются из текущего состояния неймспейса, поэтому внутри него ничего не меняется;
- дальше источником истины становится проект: удаление неймспейса больше не удаляет проект — проект создаст неймспейс заново.

Системные неймспейсы (`d8-*`, `kube-*`, `upmeter-*`, `default` и всё с лейблом `heritage: deckhouse` или `heritage: upmeter`) в проекты таким образом не превращаются — они относятся к виртуальным проектам `deckhouse`/`default` (подробнее — в разделе [«Виртуальные проекты»](#виртуальные-проекты)).

Например:

1. Создайте новый неймспейс:

   ```shell
   d8 k create ns test
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
   test        Deployed   simple                                                                    1m
   ```

Шаблон созданного проекта можно изменить на существующий.

{% alert level="warning" %}
Обратите внимание, что при смене шаблона может возникнуть конфликт ресурсов: если в чарте шаблона прописаны ресурсы, которые уже присутствуют в неймспейсе, то применить шаблон не получится.
{% endalert %}

### Создание проекта без указания шаблона

Поле `projectTemplateName` необязательно: если его не указать, используется шаблон `simple`. Такой проект состоит только из неймспейса и [стандартных полей](#стандартные-поля-проекта) (администраторы, квота) — никакие политики из шаблонов в нём не создаются. Это удобно, когда настройки не нужны или управляются другими средствами:

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
| `TemplateRolesAllowed` | Шаблон создаёт привязку к роли, [запрещённой для выдачи в проектах](#предоставление-доступа-внутри-проекта), — проект переводится в `Error`, в `message` указана роль. |
| `TemplateResourcesFiltered` | Из шаблона были отброшены объекты ResourceQuota/AuthorizationRule (см. [стандартные поля](#стандартные-поля-проекта)). Условие информационное — проект продолжает работать. |

Прочие полезные поля статуса:

- `.status.namespaces` — все неймспейсы проекта с указанием их типа (`Main`/`Additional`);
- `.status.usage` — текущее потребление квоты (заполняется при заданном `.spec.quota`);
- `.status.resources` — состояние отдельных ресурсов, созданных из шаблона.

### Служебные объекты проекта

Контроллер создаёт в неймспейсах проекта служебные объекты. Они управляются автоматически — редактировать их вручную нельзя (попытка будет отклонена):

| Объект | Где | Откуда берётся |
|--------|-----|----------------|
| `ResourceQuota/d8-project-quota` | Основной неймспейс | Поле [`.spec.quota`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-quota) проекта. |
| `ProjectRoleBinding/d8-administrators` | Основной неймспейс | Поле [`.spec.administrators`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-administrators) проекта. |
| `RoleBinding/d8:prb:<имя>` | Каждый неймспейс проекта | Разворачивание [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) с именем `<имя>`. |
| `RoleBinding/d8:cprb:<имя>` | Каждый неймспейс каждого проекта | Разворачивание [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) с именем `<имя>`. |

При удалении исходного объекта (привязки, поля квоты и т. д.) соответствующие служебные объекты удаляются автоматически.

## Виртуальные проекты

Помимо созданных пользователями проектов, в списке `d8 k get projects` всегда присутствуют два **виртуальных** проекта (лейбл `projects.deckhouse.io/virtual-project: "true"`):

- `deckhouse` — системные неймспейсы (`d8-*`, `kube-*`, `upmeter-*`, `heritage: deckhouse` / `heritage: upmeter`);
- `default` — остальные неймспейсы без своего проекта (сам неймспейс `default`).

Статус виртуального проекта заново формируется из текущего списка неймспейсов: удалённый неймспейс из этого списка пропадает. Виртуальные проекты неймспейсы не создают заново.

Виртуальные проекты нужны для полноты картины: с ними каждый неймспейс кластера относится к какому-то проекту. Управлять ими нельзя: они не редактируются, в них нельзя создавать [ProjectNamespace](/modules/multitenancy-manager/cr.html#projectnamespace) и [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding), и на них не распространяются [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding).

## Дополнительные неймспейсы проекта

Если приложению требуется несколько неймспейсов (например, отдельный под кеш или очереди), добавьте их в проект ресурсом [ProjectNamespace](/modules/multitenancy-manager/cr.html#projectnamespace). Ресурс создаётся **в основном неймспейсе проекта**. Итоговый неймспейс получает имя `<имя проекта>-<spec.name>`:

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

- **Доступ**: привязки [ProjectRoleBinding](#предоставление-доступа-внутри-проекта) и [ClusterProjectRoleBinding](#предоставление-доступа-внутри-проекта), включая автоматический доступ администраторов проекта. При добавлении нового неймспейса все существующие привязки разворачиваются в него без каких-либо действий со стороны пользователя.
- **Namespaced-объекты шаблона**: сетевая политика (`networkPolicy.mode: Isolated`) и настройка сбора логов (`logShipping`) создаются в каждом неймспейсе проекта. Сетевая изоляция при этом разрешает трафик между неймспейсами одного проекта.
- **Кластерные политики шаблона** (`OperationPolicy`, `SecurityPolicy` из `allowedUIDs`/`allowedGIDs`): выбирают неймспейсы по лейблу `projects.deckhouse.io/project`, то есть покрывают весь проект.
- **Наследуемые лейблы**: профиль безопасности подов (`security.deckhouse.io/pod-policy`), расширенный мониторинг (`extended-monitoring.deckhouse.io/enabled`), сканирование уязвимостей (`security-scanning.deckhouse.io/enabled`) и лейбл шаблона (`projects.deckhouse.io/project-template`) синхронизируются с основного неймспейса на дополнительные. Синхронизация полная: если соответствующую функцию выключили в шаблоне, лейбл снимется и с дополнительных неймспейсов. Благодаря лейблу шаблона [правила доступности кластерных ресурсов](#управление-доступом-к-cluster-wide-ресурсам) тоже действуют во всех неймспейсах проекта.

Действуют только в **основном** неймспейсе:

- квота проекта (`ResourceQuota` из [`.spec.quota`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-quota));
- дополнительные лейблы и аннотации из `namespaceMetadata` шаблона;
- аннотации размещения на узлах (из полей `nodeSelector` и `tolerations` шаблона).

### Лейблы неймспейсов проекта

| Лейбл | Основной | Дополнительные | Назначение |
|-------|:--------:|:--------------:|------------|
| `projects.deckhouse.io/project: <имя проекта>` | ✓ | ✓ | Принадлежность к проекту — общий лейбл всех неймспейсов проекта. |
| `projects.deckhouse.io/project-namespace: <spec.name>` | — | ✓ | Признак дополнительного неймспейса (имя ресурса ProjectNamespace). |
| `projects.deckhouse.io/project-template: <имя шаблона>` | ✓ | ✓ | Шаблон проекта; по нему применяются правила доступности кластерных ресурсов. |
| `heritage: multitenancy-manager` | ✓ | ✓ | Неймспейс управляется контроллером проектов: его `spec`, поле `finalizers` и лейблы из этой таблицы меняются через Project; остальные лейблы и аннотации можно менять напрямую. |
| `security.deckhouse.io/pod-policy`, `extended-monitoring.deckhouse.io/enabled`, `security-scanning.deckhouse.io/enabled` | ✓ | ✓ (наследуются) | Политики и фичи из шаблона проекта. |

Общий лейбл `projects.deckhouse.io/project` позволяет выбирать неймспейсы проекта с помощью команды `get ns`.

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

## Стандартные поля проекта

Администраторы проекта и квоты ресурсов не являются параметрами шаблона — это поля верхнего уровня ресурса [Project](/modules/multitenancy-manager/cr.html#project), работающие с любым шаблоном (включая `simple`):

- [`.spec.administrators`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-administrators) — список субъектов (`kind: User` или `kind: Group` и `name`), получающих административный доступ к проекту. Контроллер реализует этот доступ через автоматически создаваемый [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) `d8-administrators` в основном неймспейсе проекта.
- [`.spec.quota`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-quota) — набор жёстких лимитов [ResourceQuota](https://kubernetes.io/docs/concepts/policy/resource-quotas/) (например, `requests.cpu`, `limits.memory`). Контроллер поддерживает `ResourceQuota` в основном неймспейсе проекта и сообщает текущее потребление в `.status.usage`. Для `memory` и `storage` необходимо указывать единицу измерения (например, `2Gi`) — числа без единицы измерения означают байты и отклоняются.

{% alert level="warning" %}
Объекты `ResourceQuota` и `AuthorizationRule`, описанные внутри шаблонов проектов, больше не рендерятся: такие ресурсы теперь управляются исключительно через `.spec.quota` и `.spec.administrators`. Существующие шаблоны, в которых они объявлены, продолжают работать, но эти объекты отфильтровываются при рендеринге.
{% endalert %}

## Предоставление доступа внутри проекта

Чтобы выдать доступ к неймспейсам проекта пользователям, помимо администраторов проекта, используйте привязки ролей, которые ссылаются на общекластерные роли и автоматически разворачиваются во всех нужных неймспейсах проекта:

- [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) (неймспейсный, короткое имя `prb`) — выдаёт роль в рамках **одного** проекта. Создаётся в основном неймспейсе проекта. Контроллер создаёт RoleBinding в каждом неймспейсе этого проекта.
- [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) (кластерный, короткое имя `cprb`) — выдаёт роль сразу во **всех** невиртуальных проектах.

`roleRef` должен ссылаться на `ClusterRole`, имя которого начинается с одного из разрешённых префиксов (`d8:project:`, `d8:namespace:`, `d8:project-capability:`, `d8:namespace-capability:`, `d8:custom:`). Описание ролей — в [документации модуля user-authz](/modules/user-authz/).

При создании привязок действуют следующие проверки:

- **Защита от повышения привилегий**: создать привязку может только пользователь, у которого есть право привязывать (`bind`) указанную роль; право проверяется по имени роли. У администратора проекта (`d8:project:admin`) есть `bind` ровно на восемь ролей: `d8:project:viewer`, `d8:project:user`, `d8:project:manager`, `d8:project:admin`, `d8:namespace:viewer`, `d8:namespace:user`, `d8:namespace:manager` и `d8:namespace:admin`. Привязка к любой другой роли — кастомной `d8:custom:*`, capability, `d8:project:superadmin` — для администратора проекта отклоняется, даже если роль уже его собственных полномочий, потому что право `bind` на это имя ему никто не выдал. Такие привязки создаёт администратор кластера, либо он отдельной ClusterRole выдаёт администраторам проекта право `bind` на собственную роль.
- Роль должна существовать: привязка к несуществующей роли отклоняется.
- ServiceAccount в качестве субъекта ProjectRoleBinding должен принадлежать неймспейсу этого же проекта.
- Системные и подсистемные роли (`d8:system:*`, `d8:subsystem:*`), а также произвольные роли вне перечисленных префиксов через проектные привязки выдать нельзя.
- Роли с аннотацией `rbac.deckhouse.io/disabled-for-direct-use-in-projects: "true"` запрещены для выдачи в проектах. Эту аннотацию администратор кластера может поставить на роль, чтобы прекратить ее использование: при этом существующие привязки продолжают работать, но новые не создаются. Если такую роль использует шаблон проекта, проект переходит в статус `Error` с пояснением в условии `TemplateRolesAllowed`.

Привязка `d8-administrators`, создаваемая контроллером из поля [`.spec.administrators`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-administrators), управляется только контроллером — редактировать её вручную нельзя. Чтобы изменить состав администраторов, измените поле `.spec.administrators` проекта.

### Роли, доступные в RoleBinding внутри проекта

Кроме проектных привязок, внутри неймспейса проекта можно использовать и обычный RoleBinding — тогда роль действует только в этом одном неймспейсе. Но в проектах набор ролей, доступных для обычного RoleBinding, ограничен: разрешены только кластерные роли с лейблом `rbac.deckhouse.io/delegatable: "true"`. Из встроенных это роли `d8:namespace:*` и `d8:project:*`, а также роли уровней доступа упрощённой ролевой модели (`user-authz:user`, `user-authz:privileged-user`, `user-authz:editor`, `user-authz:admin`).

RoleBinding на любую другую кластерную роль (например, `cluster-admin`, системные роли или capabilities) в проекте будет отклонён с сообщением `references "<роль>" which is not available to project`. Это защита от обхода изоляции проекта через привязку к слишком широкой роли.

Чтобы использовать в проектах [собственную роль](/modules/user-authz/faq.html#создание-собственной-namespace--или-проектной-роли), добавьте на неё лейбл `rbac.deckhouse.io/delegatable: "true"`:

```shell
d8 k label clusterrole d8:custom:namespace:developer rbac.deckhouse.io/delegatable=true
```

Ограничение действует в неймспейсах всех проектов, включая те, что появились из отдельно созданных неймспейсов. Примеры:

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

Доступные поля (все — необязательные; полный справочник — [в описании ресурса ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate)):

| Поле | Что настраивает |
|------|-----------------|
| `podSecurityStandard` | Профиль безопасности подов: `Privileged`, `Baseline` или `Restricted`. |
| `networkPolicy.mode` | Сетевая изоляция: `Isolated` (трафик разрешён только внутри проекта и от системных компонентов платформы) или `NotRestricted`. |
| `features.monitoring` | Расширенный мониторинг неймспейсов проекта. |
| `features.vulnerabilityScanning` | Сканирование образов контейнеров на уязвимости. |
| `logShipping.clusterDestinationRef` | Сбор логов подов проекта в указанное хранилище (`ClusterLogDestination`). |
| `nodeSelector`, `tolerations` | Размещение подов проекта на выделенных узлах. |
| `allowedUIDs`, `allowedGIDs` | Допустимые диапазоны UID/GID контейнеров проекта. |
| `runtimeAudit.enabled` | Аудит обращений процессов проекта к ядру Linux. |
| `namespaceMetadata.labels`, `namespaceMetadata.annotations` | Дополнительные лейблы и аннотации неймспейсов проекта. |
| `resources`, `grantPolicies` | Выдача кластерных ресурсов через шаблон проекта — см. [«Управление доступом к cluster-wide-ресурсам»](#управление-доступом-к-cluster-wide-ресурсам). |
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

Параметром можно сделать двенадцать полей шаблона: `podSecurityStandard`, `networkPolicy.mode`, `features.monitoring`, `features.vulnerabilityScanning`, `logShipping.clusterDestinationRef`, `nodeSelector`, `tolerations`, `allowedUIDs`, `allowedGIDs`, `runtimeAudit.enabled`, `namespaceMetadata.labels` и `namespaceMetadata.annotations`. Вместо конкретного значения укажите `{fromParam: <имя параметра>}` и объявите параметр в `parametersSchema`. Значение не обязательно должно быть скалярным: параметром может быть и словарь (`nodeSelector`, `namespaceMetadata.labels`), и список (`tolerations`), и объект (`allowedUIDs`). Остальные поля — `title`, `description`, `resources`, `grantPolicies` и сам `parametersSchema` — принимают только конкретные значения. Тогда каждый проект задаёт своё значение в `.spec.parameters`, а если значение не задано — используется `default` из схемы.

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

Ссылки `fromParam` проверяются при создании шаблона: ссылка на параметр, не определённый в шаблоне, или параметр несовместимого типа (например, строковый параметр для булева поля) будет отклонена.

### Проверки шаблонов

Для действий над шаблонами действуют следующие правила:

- Шаблон, который используется хотя бы одним проектом, нельзя удалить.
- Изменение шаблона автоматически применяется ко всем проектам, созданным из него.
- Версия `deckhouse.io/v1alpha1` ресурса ProjectTemplate с текстовым полем `resourcesTemplate` (Helm-шаблонизация) больше не обслуживается, а в `v1alpha2` такого поля нет. Шаблон, сохранённый как `v1alpha1` с непустым `resourcesTemplate`, читается в `v1alpha2` без Helm-текста и с аннотацией `projects.deckhouse.io/legacy-helm-template: "true"`:
  - проекты такого шаблона контроллер не применяет: они переходят в состояние `Error` с условием `ProjectTemplateUsable` со значением `False`, а их объекты остаются ровно такими, какими были;
  - сам Helm-текст сохраняется в аннотации шаблона `projects.deckhouse.io/legacy-helm-template-body`, чтобы можно было прочитать, что он создавал. Текст длиннее 64 КиБ не сохраняется: все аннотации объекта вместе не могут превышать 256 КиБ;
  - чтобы вернуть проекты в рабочее состояние, перепишите шаблон на структурированные поля и в том же запросе уберите маркирующую аннотацию — `d8 k edit` и `d8 k apply` это позволяют. Убрать одну только аннотацию без переписывания шаблона нельзя: тогда шаблон станет рендерить пустой неймспейс, и Helm удалит все объекты, которые раньше создавал Helm-текст.

## Создание собственного шаблона для проекта

Шаблоны проектов по умолчанию включают базовые сценарии использования и служат примером возможностей шаблонов.

Для создания своего шаблона:

1. Возьмите за основу один из шаблонов по умолчанию, например, `default`.
1. Скопируйте его в отдельный файл, например, `my-project-template.yaml` при помощи команды:

   ```shell
   d8 k get projecttemplates default -o yaml > my-project-template.yaml
   ```

1. Отредактируйте файл `my-project-template.yaml`: внесите нужные изменения в [структурированные поля](#структурированные-шаблоны) и схему входных параметров.
1. Измените имя шаблона в поле `.metadata.name`.
1. Примените полученный шаблон командой:

   ```shell
   d8 k apply -f my-project-template.yaml
   ```

1. Проверьте доступность нового шаблона с помощью команды:

   ```shell
   d8 k get projecttemplates <ИМЯ_НОВОГО_ШАБЛОНА>
   ```

## Использование лейблов для управления ресурсами

При создании ресурсов в ProjectTemplate можно использовать специальные лейблы для управления поведением `multitenancy-manager` при обработке этих ресурсов:

### Пропуск создания лейбла `heritage: multitenancy-manager`

По умолчанию все ресурсы, созданные из ProjectTemplate, получают лейбл `heritage: multitenancy-manager`.  
Он запрещает изменение ресурсов пользователями или любым контроллером, кроме `multitenancy-manager`.  
Если необходимо разрешить изменение ресурса (например, для совместимости с другими системами, или в случае реализации собственного контроля изменения создаваемых объектов), добавьте к ресурсу лейбл `projects.deckhouse.io/skip-heritage-label`.

Пример:

{% raw %}

```yaml
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

- Будут созданы **только один раз** при создании проекта.
- **Не будут обновляться** при последующих изменениях шаблона или обновлениях.
- Не будут отслеживаться в статусе проекта.
- Получат лейблы `projects.deckhouse.io/project` и `projects.deckhouse.io/project-template`, но **не получат** лейбл `heritage: multitenancy-manager`.

{% alert level="warning" %}
После того как ресурс помечен как `unmanaged`, он будет создан при первой установке, но не будет обновляться при изменении ProjectTemplate.
После создания ресурс становится полностью независимым и должен управляться вручную.
{% endalert %}

## Реализация валидации изменений объектов с помощью пользовательского лейбла

Модуль `multitenancy-manager` использует ValidatingAdmissionPolicy для защиты ресурсов с лейблом `heritage: multitenancy-manager` от ручных изменений.  
Вы можете реализовать аналогичную валидацию для ресурсов с любым лейблом.

### Принцип работы валидации в multitenancy-manager

`multitenancy-manager` валидирует объекты с лейблом `heritage: multitenancy-manager`.  
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
       - expression: 'request.userInfo.username == "system:serviceaccount:my-namespace:my-service-account"' # Замените на ваш сервисный аккаунт.
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
   - `request.userInfo.username` — имя сервисного аккаунта, которому разрешено изменять ресурсы (замените на ваш сервисный аккаунт);
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

Модуль `multitenancy-manager` позволяет администраторам кластера определять для каждого проекта, какие
cluster-wide-ресурсы (например, StorageClass, ClusterIssuer, ClusterRole, LoadBalancerClass) можно
использовать из неймспейсов проектов, и какое значение используется по умолчанию.

Механизм работает независимо от RBAC. RBAC определяет, *кто может создавать и изменять* объекты, а механизм управления доступом к cluster-wide-ресурсам — *какие ресурсы* могут использовать эти объекты.

В механизме участвуют четыре кастомных ресурса:

- [GrantableClusterResourceDefinition](/modules/multitenancy-manager/cr.html#grantableclusterresourcedefinition) — регистрирует тип cluster-wide-ресурсов, доступом к которому можно управлять. Такие ресурсы поставляются DP или разработчиками модулей;
- [GrantableClusterResourceReference](/modules/multitenancy-manager/cr.html#grantableclusterresourcereference) — определяет, где используется зарегистрированный cluster-wide-ресурс. Например, какое поле ресурса содержит ссылку на него. Такие ресурсы поставляются модулями;
- [ClusterResourceGrantPolicy](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy) — задаёт правила доступа. Администратор кластера с помощью лейблов выбирает проекты, на которые распространяется политика, определяет разрешённые и запрещённые ресурсы, а также ресурс, используемый по умолчанию;
- [AvailableClusterResource](/modules/multitenancy-manager/cr.html#availableclusterresource) — создаваемый контроллером список cluster-wide-ресурсов, доступных проекту, который предназначен только для чтения.

Пока администратор не создал ClusterResourceGrantPolicy, доступность ресурсов определяется их регистрацией: ресурсы доступны всем проектам, если в GrantableClusterResourceDefinition задано `defaultAvailability: All` (значение по умолчанию) и ресурс не попадает под фильтры `excluded`.
Проверка доступа выполняется только для объектов в неймспейсах проектов. Если политика доступа изменяется, уже используемые существующими объектами cluster-wide-ресурсы остаются доступными для этих объектов.

Подробное описание механизма управления доступом приведено [в документации модуля `multitenancy-manager`](/modules/multitenancy-manager/#управление-доступом-к-cluster-wide-ресурсам).
