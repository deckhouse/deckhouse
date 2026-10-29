---
title: Управление проектами
permalink: ru/admin/multitenancy/project-management.html
description: Управление проектами
lang: ru
---

В Deckhouse Platform есть набор шаблонов для создания [проектов](../../glossary.html#project). Они кумулятивны: каждый следующий включает возможности предыдущего и добавляет свои. Значения параметров задаются в поле `.spec.parameters` ресурса Project:

- `simple` — минимальный шаблон, создающий только неймспейс проекта. Используйте его, когда требуется только изолированный неймспейс, управляемый как проект, а доступ и ограничения настраиваются через [стандартные поля](#стандартные-поля-проекта) и [привязки ролей проекта](#предоставление-доступа-внутри-проекта).

  Параметры:
  - `namespace.labels` и `namespace.annotations` — дополнительные лейблы и аннотации неймспейса проекта.
  - `requiredRequests` (по умолчанию `false`) — рендерится, только если параметр объявлен в схеме шаблона и равен `true`. Если `true`, у подов проекта должны быть заданы CPU и memory requests (OperationPolicy в режиме `Deny`).

- `default` — шаблон для базовых сценариев использования проектов. В дополнение к неймспейсу он настраивает сетевую изоляцию, профиль безопасности подов, [расширенный мониторинг](../../glossary.html#extended-monitoring) и доставку логов.

  Параметры (кроме перечисленных для `simple`):
  - `networkPolicy` — `Isolated` (по умолчанию) разрешает входящий трафик только из неймспейсов проекта, от Prometheus, контроллера ingress-nginx и агента service-with-healthchecks, а исходящий — только в неймспейсы проекта и к DNS в `kube-system`; `NotRestricted` разрешает весь трафик.
  - `podSecurityProfile` — профиль [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) для неймспейсов проекта: `Baseline` (по умолчанию) запрещает известные способы повышения привилегий, `Restricted` применяет максимально строгие практики, `Privileged` не ограничивает ничего. Пустая строка не задаёт профиль: проект не ставит на свои неймспейсы лейбл `security.deckhouse.io/pod-policy`, и к ним применяется [политика по умолчанию](/modules/admission-policy-engine/configuration.html#parameters-podsecuritystandards-defaultpolicy) модуля `admission-policy-engine`.
  - `extendedMonitoringEnabled` (по умолчанию `true`) — алерты о недоступности и перезапусках контроллеров, ошибках 5xx в ingress-nginx и нехватке свободного места на persistent volume'ах проекта.
  - `clusterLogDestinationName` — имя ресурса ClusterLogDestination, в который отправлять логи проекта. Если не задан, логи проекта никуда не отправляются.
  - `requiredRequests` (в этом шаблоне по умолчанию `true`) — если `true`, у подов проекта должны быть заданы CPU и memory requests (OperationPolicy в режиме Deny). При adopt существующего неймспейса параметр выставляется в `false`, чтобы не блокировать уже работающие нагрузки.

- `secure` — включает все возможности шаблона `default`, а также ограничение пользователей и групп внутри контейнеров, аудит их обращений к ядру и [сканирование образов на уязвимости](../../glossary.html#vulnerability-scanning).

  Параметры (кроме перечисленных для `default`):
  - `allowedUIDs` и `allowedGIDs` — диапазоны (`min`, `max`) идентификаторов, допустимых для пользователей и групп внутри контейнеров проекта. См. [Security Context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod).
  - `runtimeAuditEnabled` (по умолчанию `false`) — правила аудита обращений к ядру, выявляющие вредоносную активность. Работают, только если задан диапазон UID/GID.
  - `securityScanningEnabled` (по умолчанию `true`) — периодическое сканирование запускаемых образов на известные уязвимости (CVE) средствами Trivy, раз в 24 часа.

- `secure-with-dedicated-nodes` — включает все возможности шаблона `secure`, а также размещение подов [основного неймспейса проекта](../../glossary.html#main-project-namespace) на выделенных узлах.

  Параметры (кроме перечисленных для `secure`). Если задан `dedicatedNodes`, в нём должно быть хотя бы одно из двух полей. Без `dedicatedNodes` поды не размещаются на выделенных узлах:
  - `dedicatedNodes.nodeSelector` — селектор узлов основного неймспейса. Он объединяется с селектором узлов каждого пода в этом неймспейсе, а под с конфликтующим селектором отклоняется.
  - `dedicatedNodes.defaultTolerations` — tolerations в формате `spec.tolerations` пода. **Добавляются** к подам, создаваемым в основном неймспейсе.

Шаблоны `default`, `secure` и `secure-with-dedicated-nodes` описаны в [структурированном виде](#структурированные-шаблоны) (`deckhouse.io/v1alpha2`). Шаблон `simple` — минимальный структурированный шаблон, который создаёт неймспейс, задаёт его лейблы и аннотации из параметров проекта, а при `requiredRequests: true` создаёт и OperationPolicy.

Точный набор параметров смотрите в шаблоне, установленном в вашем кластере, — он соответствует версии платформы.

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
   API ресурса Project обслуживается как `deckhouse.io/v1alpha3`. Манифесты `v1alpha2` продолжают работать: вебхук конвертации автоматически переносит `parameters.administrators` и `parameters.resourceQuota` в стандартные поля `.spec.administrators` и `.spec.quota`. Если хотя бы один администратор не может быть добавлен в `.spec.administrators` (например, у него задан `accessLevel`), весь список остаётся в `parameters`, и `.spec.administrators` не заполняется. То же относится к квоте, в которой есть что-то, кроме значений ресурсов (например, `scopes`), к квоте, в которой где-либо есть число `0`, а также к пустому списку или пустой квоте.
   Версия `deckhouse.io/v1alpha1` больше не обслуживается, и API-сервер отклоняет манифест с `apiVersion: deckhouse.io/v1alpha1`. Перед применением такого манифеста замените `apiVersion` на `deckhouse.io/v1alpha3`, переименуйте `spec.projectTypeName` в `spec.projectTemplateName` (ProjectTemplate с таким именем должен существовать) и `spec.template` в `spec.parameters`, а `administrators` и `resourceQuota` перенесите из параметров в `.spec.administrators` и `.spec.quota`.
   {% endalert %}

1. Для проверки статуса [проекта](../../glossary.html#project) выполните команду:

   ```shell
   d8 k get projects my-project
   ```

   Успешно созданный проект должен отображаться в статусе `Deployed` (синхронизирован). Если отображается статус `Error` (ошибка), добавьте аргумент `-o yaml` к команде (например, `d8 k get projects my-project -o yaml`) для получения более подробной информации о причине ошибки.

### Автоматическое создание проекта для неймспейса

Неймспейс, созданный напрямую (например, с использованием команды `d8 k create ns test`), автоматически становится проектом с тем же именем — никакая аннотация для этого не требуется:

- шаблон подбирается по тому, что на неймспейсе уже есть: `secure` — при лейбле `security-scanning.deckhouse.io/enabled`, `default` — при `security.deckhouse.io/pod-policy` или `extended-monitoring.deckhouse.io/enabled`, иначе `simple`;
- параметры проекта заполняются из текущего состояния неймспейса, поэтому внутри него ничего не меняется: [сетевая политика](../../glossary.html#network-policy) остаётся неограниченной, а Pod Security Standard сохраняет то значение, которое уже было;
- дальше источником истины становится проект, и он редактируется как любой другой. Удалить неймспейс проекта напрямую нельзя: запрос отклоняется с сообщением `Manual modification is forbidden`. Чтобы убрать окружение, удалите Project: `d8 k delete project <NAME>` удаляет вместе с ним неймспейс и возвращает управление, когда неймспейс исчезнет. Удалить Project может только пользователь, которому разрешено удалять его неймспейс (подробнее — в разделе [«Дополнительные неймспейсы проекта»](#дополнительные-неймспейсы-проекта)).

Создать неймспейс напрямую пользователи могут, только пока параметр [`allowNamespacesWithoutProjects`](/modules/multitenancy-manager/configuration.html#parameters-allownamespaceswithoutprojects) равен `true` (значение по умолчанию). При `false` напрямую неймспейсы создают только администраторы кластера (`system:masters`, `system:sudouser`), Deckhouse, контроллер модуля и [системные компоненты](../../glossary.html#system-component). Остальные пользователи создают Project или ProjectNamespace.

[Системные неймспейсы](../../glossary.html#system-namespace) (`d8-*`, `kube-*`, `upmeter-*`, `default` и всё с лейблом `heritage: deckhouse` или `heritage: upmeter`) в проекты таким образом не превращаются — они относятся к виртуальным проектам `deckhouse`/`default` (подробнее — в разделе [«Виртуальные проекты»](#виртуальные-проекты)). Неймспейс длиннее 61 символа тоже не превращается в проект: это лимит имени Project.

Неймспейс, который из-за лейбла мониторинга или сканирования получает шаблон `default` или `secure` и у которого нет лейбла `security.deckhouse.io/pod-policy`, получает `podSecurityProfile: ""`. Проект не ставит на него этот лейбл, и к нему по-прежнему применяется [политика по умолчанию](/modules/admission-policy-engine/configuration.html#parameters-podsecuritystandards-defaultpolicy) модуля `admission-policy-engine`.

Неймспейс, который уже принадлежит другому Helm-релизу (его аннотация `meta.helm.sh/release-name` указывает не на релиз проекта), не перехватывается: проект получает условие `HelmOwnership` с именем этого релиза и повторяет попытку, а неймспейс остаётся как есть. Чтобы проект продолжил работу, удалите чужой релиз или его аннотации владения.

Уже существующие RoleBinding и AuthorizationRule внутри неймспейса после перехода в проект продолжают работать. Admin неймспейса **не** копируется в `.spec.administrators` и **не** становится `d8:project:admin`. Чтобы сделать его администратором проекта, добавьте его в `.spec.administrators` или создайте ProjectRoleBinding.

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

   ```console
   NAME        STATE      PROJECT TEMPLATE   DESCRIPTION                                            AGE
   deckhouse   Deployed   virtual            This is a virtual project                              181d
   default     Deployed   virtual            This is a virtual project                              181d
   test        Deployed   simple                                                                    1m
   ```

Чтобы сменить шаблон такого проекта, укажите другой шаблон в `.spec.projectTemplateName`.

{% alert level="warning" %}
Если в неймспейсе уже есть объект того же типа и с тем же именем, что и объект, который создаёт новый шаблон, например NetworkPolicy `isolated`, проект переходит в состояние `Error`, а условие `ResourcesUpgraded` называет этот объект. Сначала удалите или переименуйте его.
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
- имя не может быть `default` или `deckhouse` — эти имена заняты [виртуальными проектами](#виртуальные-проекты);
- имя не может быть длиннее 61 символа;
- имя не может совпадать с именем существующего неймспейса или неймспейса, который занял ресурс [ProjectNamespace](#дополнительные-неймспейсы-проекта) (имя занято с момента создания ProjectNamespace, даже если контроллер ещё не создал неймспейс);
- имя не может совпадать с именем неймспейса, чьи ServiceAccount всё ещё указаны среди субъектов ClusterProjectRoleBinding или ProjectRoleBinding проекта, к имени которого новое имя добавляет дефис и суффикс (`foo` для `foo-extra`). Такие субъекты часто остаются от удалённого дополнительного неймспейса. Сначала удалите их из привязки.

Проект может называться как другой проект с дефисом и суффиксом (`foo-bar` рядом с `foo`). Это отдельный проект со своим неймспейсом, а не [дополнительный неймспейс проекта](../../glossary.html#additional-project-namespace) `foo`, поэтому шаблон, администраторы, квота и привязки проекта `foo` на него не распространяются. Вебхук пропускает такой проект с предупреждением, а контроллер отправляет Warning-событие `NamedLikeAdditionalNamespace` на новый проект. Все такие пары проектов показывает алерт [`MultitenancyManagerProjectNamedLikeAdditionalNamespace`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#multitenancy-manager-multitenancymanagerprojectnamedlikeadditionalnamespace). Чтобы добавить неймспейс в проект, создайте ресурс [ProjectNamespace](#дополнительные-неймспейсы-проекта).

## Статус и диагностика проекта

Поле `.status.state` [проекта](../../glossary.html#project) принимает значение `Deployed` (все ресурсы проекта синхронизированы или объекты из `manifests` ждут своей привязки) либо `Error`. Причина ошибки описывается в условиях (`.status.conditions`):

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
| `ManifestsApplied` | Объекты из [`manifests`](/modules/multitenancy-manager/usage.html#описание-других-объектов-в-manifests) шаблона не применены. В `message` указано, чего они ждут, почему не применён ни один из них, какие уже существующие объекты они не трогают или какие объекты отклонил API-сервер, с кодом каждого отказа (например, `403 Forbidden`). |
| `HelmOwnership` | Неймспейс проекта принадлежит другому Helm-релизу. В `message` указан этот релиз (подробнее — в разделе [«Автоматическое создание проекта для неймспейса»](#автоматическое-создание-проекта-для-неймспейса)). |
| `NamespaceDeleted` | Проект удаляется, а его неймспейс ещё не удалён. В `message` указано, с какого момента идёт удаление и что осталось в неймспейсе. |

Условие `HandoverPending` читается наоборот. Оно равно `True`, пока объекты AuthorizationRule и ResourceQuota, сохранённые от Helm-текста, ждут стандартных полей, которые их заменяют, и удаляется, когда эти объекты удалены (см. [«Переход проекта с Helm-текста»](/modules/multitenancy-manager/usage.html#переход-проекта-с-helm-текста)).

Прочие полезные поля статуса:

- `.status.namespaces` — все неймспейсы проекта с указанием их типа (`Main`/`Additional`);
- `.status.usage` — текущее потребление квоты (заполняется при заданном `.spec.quota`);
- `.status.resources` — состояние отдельных ресурсов, созданных из шаблона;
- `.status.keptObjects` — объекты Helm-текста, которые [остались на месте](/modules/multitenancy-manager/usage.html#переход-проекта-с-helm-текста) вместо удаления;
- `.status.manifestObjects` — объекты, применённые из [`manifests`](/modules/multitenancy-manager/usage.html#описание-других-объектов-в-manifests) шаблона.

### Служебные объекты проекта

Контроллер создаёт в неймспейсах проекта служебные объекты. Они управляются автоматически — редактировать их вручную нельзя (попытка будет отклонена):

| Объект | Где | Откуда берётся |
|--------|-----|----------------|
| `ResourceQuota/d8-project-quota` | Основной неймспейс | Поле [`.spec.quota`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-quota) проекта. |
| `ProjectRoleBinding/d8-administrators` | Основной неймспейс | Поле [`.spec.administrators`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-administrators) проекта. |
| `ProjectRoleBinding/d8-manifests` | Основной неймспейс | Поле [`manifests`](/modules/multitenancy-manager/usage.html#описание-других-объектов-в-manifests) шаблона проекта. Выдаёт роль `d8:project:admin` пользователю, от имени которого применяются эти объекты. |
| `RoleBinding/d8:prb:<имя>` | Каждый неймспейс проекта | Разворачивание [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) с именем `<имя>`. |
| `RoleBinding/d8:cprb:<имя>` | Каждый неймспейс каждого проекта | Разворачивание [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) с именем `<имя>`. |

При удалении исходного объекта (привязки, поля квоты и т. д.) соответствующие служебные объекты удаляются автоматически.

## Виртуальные проекты

Помимо созданных пользователями проектов, в списке `d8 k get projects` всегда присутствуют два **виртуальных** проекта (лейбл `projects.deckhouse.io/virtual-project: "true"`):

- `deckhouse` — [системные неймспейсы](../../glossary.html#system-namespace) (`d8-*`, `kube-*`, `upmeter-*`, `heritage: deckhouse` / `heritage: upmeter`);
- `default` — остальные неймспейсы без своего проекта (сам неймспейс `default`).

Статус виртуального проекта заново формируется из текущего списка неймспейсов: удалённый неймспейс из этого списка пропадает. Виртуальные проекты неймспейсы не создают заново.

Виртуальные проекты нужны для полноты картины: с ними каждый неймспейс кластера относится к какому-то проекту. Управлять ими нельзя: они не редактируются, в них нельзя создавать [ProjectNamespace](/modules/multitenancy-manager/cr.html#projectnamespace) и [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding), и на них не распространяются [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding). Их шаблон `virtual` зарезервирован за ними: пользовательский проект с `projectTemplateName: virtual` вебхук отклонит. Политики ClusterResourceGrantPolicy к их неймспейсам тоже не применяются, даже если селекторы политики выбирают виртуальный Project, потому что [правила доступности кластерных ресурсов](#управление-доступом-к-cluster-wide-ресурсам) действуют только в неймспейсах с лейблом `projects.deckhouse.io/project`, а на неймспейсах виртуальных проектов его нет.

Объекта ProjectTemplate с именем `virtual` нет, и вебхук не даст его создать. Если такой шаблон уже есть в кластере, он не используется ни одним проектом, и его можно удалить.

## Дополнительные неймспейсы проекта

Если приложению требуется несколько неймспейсов (например, отдельный под кеш или очереди), добавьте их в проект ресурсом [ProjectNamespace](/modules/multitenancy-manager/cr.html#projectnamespace). Ресурс создаётся **в [основном неймспейсе проекта](../../glossary.html#main-project-namespace)**. Итоговый неймспейс получает имя `<имя проекта>-<spec.name>`:

```yaml
apiVersion: deckhouse.io/v1alpha3
kind: ProjectNamespace
metadata:
  name: cache
  namespace: my-project
spec:
  name: cache   # Будет создан неймспейс my-project-cache.
```

Проверить состав [проекта](../../glossary.html#project) можно по его статусу:

```shell
d8 k get project my-project -o jsonpath='{.status.namespaces}'
```

Правила работы с ProjectNamespace:

- Поле `spec.name` неизменяемо: чтобы переименовать неймспейс, удалите ресурс и создайте новый.
- Итоговое имя `<имя проекта>-<spec.name>` не может быть длиннее 63 символов (ограничение Kubernetes на имена неймспейсов).
- Создавать ProjectNamespace можно только в основном неймспейсе проекта — «вложить» его в дополнительный неймспейс или чужой проект нельзя.
- Итоговое имя должно быть свободно. Запрос отклоняется, если неймспейс с таким именем существует и не относится к этому проекту, если так называется проект или если это имя уже занял другой ресурс ProjectNamespace (например, ProjectNamespace `a-backend` проекта `team` и ProjectNamespace `backend` проекта `team-a` претендуют на одно имя `team-a-backend`).
- Запрос также отклоняется, если ServiceAccount из неймспейса с итоговым именем всё ещё указаны среди субъектов ClusterProjectRoleBinding или ProjectRoleBinding другого проекта, к имени которого итоговое имя добавляет дефис и суффикс. Сначала удалите эти субъекты из привязки.
- При удалении ресурса ProjectNamespace удаляется его неймспейс. При удалении проекта удаляются все его неймспейсы.
- Удалить проект может только пользователь, которому разрешено удалять каждый его неймспейс (глагол `delete` для `namespaces`). Иначе запрос отклоняется, и в отказе назван неймспейс, который пользователю удалять нельзя.

### Что распространяется на дополнительные неймспейсы

Автоматически действует во **всех** неймспейсах проекта (и в основном, и в дополнительных):

- **Доступ**: привязки [ProjectRoleBinding](#предоставление-доступа-внутри-проекта) и [ClusterProjectRoleBinding](#предоставление-доступа-внутри-проекта), включая автоматический доступ администраторов проекта. При добавлении нового неймспейса все существующие привязки разворачиваются в него без каких-либо действий со стороны пользователя.
- **Namespaced-объекты шаблона**: [сетевая политика](../../glossary.html#network-policy) (`networkPolicy.mode: Isolated`) и настройка сбора логов (`logShipping`) создаются в каждом неймспейсе проекта. Сетевая изоляция при этом разрешает трафик между неймспейсами одного проекта. Helm-шаблон `resourcesTemplate` создаёт свои объекты в основном неймспейсе, если в объекте не указан другой неймспейс проекта. Так же поступает [`manifests`](/modules/multitenancy-manager/usage.html#описание-других-объектов-в-manifests), объекты которого не применяются, если хотя бы один из них указывает неймспейс вне проекта.
- **Кластерные политики шаблона** (`OperationPolicy`, `SecurityPolicy` из `allowedUIDs`/`allowedGIDs`): выбирают неймспейсы по лейблу `projects.deckhouse.io/project`, то есть покрывают весь проект.
- **Наследуемые лейблы**: профиль безопасности подов (`security.deckhouse.io/pod-policy`), [расширенный мониторинг](../../glossary.html#extended-monitoring) (`extended-monitoring.deckhouse.io/enabled`), сканирование уязвимостей (`security-scanning.deckhouse.io/enabled`), лейбл шаблона (`projects.deckhouse.io/project-template`) и лейблы, которые шаблон задаёт через `namespaceMetadata.labels`, синхронизируются с основного неймспейса на дополнительные. Синхронизация полная: если функцию выключили или лейбл убрали в шаблоне, лейбл снимется и с дополнительных неймспейсов. Лейблы шаблона перечислены в аннотации `projects.deckhouse.io/template-labels` каждого неймспейса проекта; их меняют через Project или его ProjectTemplate, а прямая правка такого лейбла на дополнительном неймспейсе откатывается. Благодаря лейблу шаблона [правила доступности кластерных ресурсов](#управление-доступом-к-cluster-wide-ресурсам) тоже действуют во всех неймспейсах проекта.

Действуют только в **основном** неймспейсе:

- квота проекта (`ResourceQuota` из [`.spec.quota`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-quota));
- аннотации из `namespaceMetadata` шаблона;
- лейблы и аннотации, поставленные на основной неймспейс напрямую или GitOps-инструментом, в том числе те, что были на неймспейсе до того, как он [стал проектом](#автоматическое-создание-проекта-для-неймспейса), кроме перечисленных выше лейблов профиля безопасности, мониторинга и сканирования;
- лейблы отслеживания GitOps-инструментов (`app.kubernetes.io/instance`, `argocd.argoproj.io/*`, `kustomize.toolkit.fluxcd.io/*`, `helm.toolkit.fluxcd.io/*`, `kapp.k14s.io/*`), даже если их задаёт шаблон;
- аннотации размещения на узлах (из полей `nodeSelector` и `tolerations` шаблона).

Когда неймспейс [становится проектом автоматически](#автоматическое-создание-проекта-для-неймспейса), его собственные лейблы и аннотации не копируются в Project, поэтому [дополнительные неймспейсы проекта](../../glossary.html#additional-project-namespace) их не наследуют. Исключение составляют лейблы профиля безопасности, мониторинга и сканирования: при переходе в проект они становятся параметрами проекта. Чтобы другой лейбл такого неймспейса был и на дополнительных неймспейсах, задайте его в `spec.parameters.namespace.labels` ресурса Project либо поставьте его на каждый дополнительный неймспейс напрямую.

### Лейблы неймспейсов проекта

| Лейбл | Основной | Дополнительные | Назначение |
|-------|:--------:|:--------------:|------------|
| `projects.deckhouse.io/project: <имя проекта>` | ✓ | ✓ | Принадлежность к проекту — общий лейбл всех неймспейсов проекта. |
| `projects.deckhouse.io/project-namespace: <spec.name>` | — | ✓ | Признак дополнительного неймспейса (имя ресурса ProjectNamespace). |
| `projects.deckhouse.io/project-template: <имя шаблона>` | ✓ | ✓ | Шаблон проекта; по нему применяются правила доступности кластерных ресурсов. |
| `heritage: multitenancy-manager` | ✓ | ✓ | Неймспейс управляется контроллером проектов: его `spec`, поле `finalizers` и лейблы из этой таблицы меняются через Project; остальные лейблы и аннотации можно менять напрямую. |
| `security.deckhouse.io/pod-policy`, `extended-monitoring.deckhouse.io/enabled`, `security-scanning.deckhouse.io/enabled` | ✓ | ✓ (наследуются) | Политики и фичи из шаблона проекта. |
| Лейблы из `namespaceMetadata.labels` шаблона | ✓ | ✓ (наследуются) | Лейблы шаблона проекта. Перечислены в аннотации `projects.deckhouse.io/template-labels` неймспейса и меняются через Project или его ProjectTemplate. |
| Любой другой лейбл, поставленный на неймспейс напрямую или GitOps-инструментом | — | — | Модуль его не ставит. Релиз проекта не откатывает и не возвращает его, дополнительные неймспейсы его не наследуют. |

Общий лейбл `projects.deckhouse.io/project` позволяет выбирать неймспейсы проекта с помощью команды `get ns`.

Получить все неймспейсы проекта (основной + дополнительные):

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

Администраторы [проекта](../../glossary.html#project) и квоты ресурсов не являются параметрами шаблона — это поля верхнего уровня ресурса [Project](/modules/multitenancy-manager/cr.html#project), работающие с любым шаблоном (включая `simple`):

- [`.spec.administrators`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-administrators) — список субъектов (`kind: User` или `kind: Group` и `name`), получающих административный доступ к проекту. Контроллер реализует этот доступ через автоматически создаваемый [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) `d8-administrators` в [основном неймспейсе проекта](../../glossary.html#main-project-namespace). Имя не может начинаться с `system:multitenancy-manager:` — префикса пользователей, от имени которых действует контроллер.
- [`.spec.quota`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-quota) — набор жёстких лимитов [ResourceQuota](https://kubernetes.io/docs/concepts/policy/resource-quotas/) (например, `requests.cpu`, `limits.memory`). Контроллер поддерживает `ResourceQuota` в основном неймспейсе проекта и сообщает текущее потребление в `.status.usage`. Для `memory` и `storage` необходимо указывать единицу измерения (например, `2Gi`). Числа без единицы измерения означают байты, и все они, кроме 0, отклоняются. Обновление, которое оставляет такое значение прежним, принимается с предупреждением, как и создание через `deckhouse.io/v1alpha2` с такими числами в `parameters.resourceQuota`.

{% alert level="warning" %}
Helm-шаблон `resourcesTemplate`, который рендерит ResourceQuota или AuthorizationRule, продолжает их рендерить наряду с ResourceQuota и ProjectRoleBinding, которые создают стандартные поля. Под в таком проекте должен укладываться в каждую ResourceQuota своего неймспейса, а доступ, который выдают такие AuthorizationRule, добавляется к доступу из стандартных полей.
{% endalert %}

## Предоставление доступа внутри проекта

Чтобы выдать доступ к неймспейсам проекта пользователям, помимо администраторов проекта, используйте привязки ролей, которые ссылаются на общекластерные роли и автоматически разворачиваются во всех нужных неймспейсах проекта:

- [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) (неймспейсный, короткое имя `prb`) — выдаёт роль в рамках **одного** проекта. Создаётся в основном неймспейсе проекта. Контроллер создаёт RoleBinding в каждом неймспейсе этого проекта.
- [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) (кластерный, короткое имя `cprb`) — выдаёт роль сразу во **всех** невиртуальных проектах.

`roleRef` должен ссылаться на `ClusterRole`, имя которого начинается с одного из разрешённых префиксов (`d8:project:`, `d8:namespace:`, `d8:project-capability:`, `d8:namespace-capability:`, `d8:custom:`). Описание ролей — в [документации модуля user-authz](/modules/user-authz/).

При создании привязок действуют следующие проверки:

- **Защита от повышения привилегий**: создать привязку может только пользователь, у которого есть право привязывать (`bind`) указанную роль; право проверяется по имени роли. У администратора проекта (`d8:project:admin`) есть `bind` ровно на восемь ролей: `d8:project:viewer`, `d8:project:user`, `d8:project:manager`, `d8:project:admin`, `d8:namespace:viewer`, `d8:namespace:user`, `d8:namespace:manager` и `d8:namespace:admin`. Привязка к любой другой роли — кастомной `d8:custom:*`, capability, `d8:project:superadmin` — для администратора проекта отклоняется, даже если эта роль уже входит в его полномочия, потому что право `bind` на это имя ему никто не выдал. Такие привязки создаёт администратор кластера, либо он отдельной ClusterRole выдаёт администраторам проекта право `bind` на собственную роль.
- Роль должна существовать: привязка к несуществующей роли отклоняется.
- ServiceAccount в качестве субъекта ProjectRoleBinding должен принадлежать неймспейсу этого же проекта: основному, дополнительному или неймспейсу, который занял ресурс ProjectNamespace этого проекта. Неймспейс отдельного проекта, который только называется как дополнительный (`<проект>-*`), не подходит.
- Системные и подсистемные роли (`d8:system:*`, `d8:subsystem:*`), а также произвольные роли вне перечисленных префиксов через проектные привязки выдать нельзя.
- Роли с аннотацией `rbac.deckhouse.io/disabled-for-direct-use-in-projects: "true"` запрещены для выдачи в проектах. Эту аннотацию администратор кластера может поставить на роль, чтобы прекратить её использование: при этом существующие привязки продолжают работать, но новые не создаются. Если такую роль использует шаблон проекта, проект переходит в статус `Error` с пояснением в условии `TemplateRolesAllowed`.

Контроллер проверяет ServiceAccount по тому же правилу каждый раз, когда обновляет RoleBinding привязки ProjectRoleBinding, а также когда неймспейс появляется, удаляется или переходит в другой проект. Субъекты, которые не прошли проверку, остаются в `.spec.subjects`, но прав не получают: условие `SubjectsBound` привязки принимает значение `False` и перечисляет их. Когда неймспейс снова относится к проекту, субъекты снова получают права без изменения привязки.

Привязка `d8-administrators`, создаваемая контроллером из поля [`.spec.administrators`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-administrators), управляется только контроллером — редактировать её вручную нельзя. Чтобы изменить состав администраторов, измените поле `.spec.administrators` проекта. Описанное выше правило для ServiceAccount к этой привязке не применяется.

Следующие привязки выдают пользователю `viewer@example.com` роль `d8:project:viewer` во всех неймспейсах проекта `my-project`, а группе `platform` — ту же роль во всех проектах, кроме виртуальных:

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

### Роли, доступные в RoleBinding внутри проекта

Кроме проектных привязок, внутри неймспейса [проекта](../../glossary.html#project) можно использовать и обычный RoleBinding — тогда роль действует только в этом одном неймспейсе. Но в проектах набор ролей, доступных для обычного RoleBinding, ограничен: разрешены только кластерные роли с лейблом `rbac.deckhouse.io/delegatable: "true"`. Из встроенных это роли `d8:namespace:*` и `d8:project:*`, а также роли уровней доступа упрощённой [ролевой модели](../../glossary.html#rbac) (`user-authz:user`, `user-authz:privileged-user`, `user-authz:editor`, `user-authz:admin`).

RoleBinding на любую другую кластерную роль (например, `cluster-admin`, системные роли или capabilities) в проекте будет отклонён с сообщением `references "<роль>" which is not available to project`. Это защита от обхода изоляции проекта через привязку к слишком широкой роли.

Чтобы использовать в проектах [собственную роль](/modules/user-authz/faq.html#создание-собственной-namespace--или-проектной-роли), добавьте на неё лейбл `rbac.deckhouse.io/delegatable: "true"`:

```shell
d8 k label clusterrole d8:custom:namespace:developer rbac.deckhouse.io/delegatable=true
```

Ограничение действует в неймспейсах всех проектов, включая те, что появились из отдельно созданных неймспейсов.

## Структурированные шаблоны

Начиная с API-версии `deckhouse.io/v1alpha2`, шаблон проекта описывается **структурированными полями** — вместо текстового Helm-шаблона вы декларативно указываете, какие настройки получат неймспейсы проекта. Контроллер сам создаёт из этих полей нужные объекты ([сетевые политики](../../glossary.html#network-policy), политики безопасности, настройки сбора логов и т. д.) в каждом неймспейсе проекта и поддерживает их в актуальном состоянии. Другие объекты неймспейсов проекта описываются в [`manifests`](/modules/multitenancy-manager/usage.html#описание-других-объектов-в-manifests).

Доступные поля (все — необязательные; полный справочник — [в описании ресурса ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate)):

| Поле | Что настраивает |
|------|-----------------|
| `podSecurityStandard` | Профиль безопасности подов: `Privileged`, `Baseline` или `Restricted` |
| `networkPolicy.mode` | Сетевая изоляция: `Isolated` (трафик разрешён только внутри проекта и от [системных компонентов](../../glossary.html#system-component) платформы) или `NotRestricted` |
| `features.monitoring` | [Расширенный мониторинг](../../glossary.html#extended-monitoring) неймспейсов проекта |
| `features.vulnerabilityScanning` | Сканирование образов контейнеров на уязвимости |
| `logShipping.clusterDestinationRef` | Сбор логов подов проекта в указанное хранилище (`ClusterLogDestination`) |
| `nodeSelector`, `tolerations` | Размещение подов основного неймспейса на выделенных узлах |
| `allowedUIDs`, `allowedGIDs` | Допустимые диапазоны UID/GID контейнеров проекта |
| `runtimeAudit.enabled` | Аудит обращений процессов проекта к ядру Linux |
| `namespaceMetadata.labels`, `namespaceMetadata.annotations` | Дополнительные лейблы и аннотации неймспейсов проекта. Лейблы, которыми управляет модуль, здесь задать нельзя: шаблон или Project с таким лейблом отклоняется |
| `resources`, `grantPolicies` | Выдача кластерных ресурсов через шаблон проекта — см. [«Управление доступом к cluster-wide-ресурсам»](#управление-доступом-к-cluster-wide-ресурсам) |
| `manifests` | Другие объекты в неймспейсах проекта, которые применяются с правами администратора проекта. Подробнее — в разделе [«Описание других объектов в manifests»](/modules/multitenancy-manager/usage.html#описание-других-объектов-в-manifests) |
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

[Проект](../../glossary.html#project), использующий такой шаблон:

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

Ссылки `fromParam` проверяются при создании и изменении шаблона: ссылка на параметр, не определённый в шаблоне, или параметр несовместимого типа (например, строковый параметр для булева поля) будет отклонена.

Параметр `requiredRequests` контроллер читает по имени, без `fromParam`. OperationPolicy в режиме `Deny`, которая требует задавать CPU и memory requests у подов во всех неймспейсах проекта, создаётся, только если шаблон объявляет параметр в `parametersSchema` с типом `boolean` и для проекта он равен `true`. Шаблон, в котором параметр не объявлен, такую политику не создаёт, даже если проект задаёт `requiredRequests` необъявленным ключом.

### Проверки шаблонов

Для действий над шаблонами применяются следующие правила:

- Шаблон нельзя удалить, пока его использует хотя бы один проект или пока он ещё применён хотя бы к одному неймспейсу проекта. Если проект перешёл на другой шаблон, старый шаблон остаётся, пока контроллер не применит новый.
- Изменение шаблона автоматически применяется ко всем проектам, созданным из него.
- Существующий шаблон может описывать свои объекты Helm-текстом в поле [`resourcesTemplate`](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha2-spec-resourcestemplate) вместо структурированных полей. Эта форма устарела, и поле `resourcesTemplate` будет удалено вместе с `deckhouse.io/v1alpha1` в одном из следующих релизов. Алерт `MultitenancyManagerHelmProjectTemplate` называет каждый шаблон, кроме встроенных, который всё ещё рендерится из Helm-текста. Описывайте новые шаблоны структурированными полями и [`manifests`](/modules/multitenancy-manager/usage.html#описание-других-объектов-в-manifests), а существующие переведите так, как описано в разделе [«Перевод шаблона с Helm-текста»](/modules/multitenancy-manager/usage.html#перевод-шаблона-с-helm-текста).
- Шаблон не может получить новое `resourcesTemplate` ни через одну из версий. Создание шаблона с непустым `resourcesTemplate` и добавление непустого `resourcesTemplate` в шаблон, у которого это поле пустое, отклоняются. Шаблон, у которого есть Helm-текст, сохраняет его, и текст можно менять через любую из двух версий. Поэтому шаблон с Helm-текстом нельзя создать заново, например из манифеста, выгруженного из другого кластера, из резервной копии или из Git после удаления шаблона. Сначала переведите такой шаблон на структурированные поля и `manifests`.
- Версия `deckhouse.io/v1alpha1` ресурса ProjectTemplate обслуживается и помечена как устаревшая, и API-сервер возвращает предупреждение на каждый запрос через неё. Шаблон создаётся только через `deckhouse.io/v1alpha2`, а создание через `deckhouse.io/v1alpha1` отклоняется. Шаблоны, созданные через `deckhouse.io/v1alpha1` раньше, читаются, меняются и удаляются через неё, как и прежде. Изменение через `v1alpha1` шаблона, в котором есть поля, которых нет в `v1alpha1`, например `title`, `grantPolicies`, `podSecurityStandard` или `manifests`, отклоняется, потому что оно стёрло бы их. Меняйте такой шаблон через `deckhouse.io/v1alpha2`.

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
1. Удалите из файла лейбл `heritage`, поля `metadata.resourceVersion`, `metadata.uid`, `metadata.creationTimestamp`, `metadata.generation`, `metadata.managedFields` и раздел `status`.
1. Примените полученный шаблон командой:

   ```shell
   d8 k apply -f my-project-template.yaml
   ```

1. Проверьте доступность нового шаблона с помощью команды:

   ```shell
   d8 k get projecttemplates <ИМЯ_НОВОГО_ШАБЛОНА>
   ```

## Лейблы для объектов Helm-текста

Следующие лейблы меняют то, как модуль обрабатывает объекты Helm-текста в `resourcesTemplate`. Такой текст есть только у существующих шаблонов, потому что ни один шаблон не может получить новое `resourcesTemplate`. Объекты из [`manifests`](/modules/multitenancy-manager/usage.html#описание-других-объектов-в-manifests) не получают лейбл `heritage: multitenancy-manager`, и модуль не читает на них эти лейблы.

### Пропуск лейбла heritage

По умолчанию каждый объект Helm-текста получает лейбл `heritage: multitenancy-manager`.
Он запрещает изменение объекта пользователями или любым контроллером, кроме `multitenancy-manager`.
Чтобы разрешить изменение объекта (например, для совместимости с другими системами или чтобы самостоятельно контролировать создаваемые объекты), добавьте на него лейбл `projects.deckhouse.io/skip-heritage-label`.

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

В этом случае объект получает лейблы `projects.deckhouse.io/project` и `projects.deckhouse.io/project-template`, но не получает лейбл `heritage: multitenancy-manager`.

### Исключение объектов из управления

Чтобы исключить объект из управления модулем (например, если он управляется вручную или другим контроллером), добавьте на него лейбл `projects.deckhouse.io/unmanaged`.

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

Объекты с лейблом `projects.deckhouse.io/unmanaged`:

- создаются **только один раз**, при первой установке релиза [проекта](../../glossary.html#project), поэтому объект, добавленный в шаблон позже, в существующих проектах не создаётся;
- **не обновляются** при изменении шаблона;
- не отслеживаются в статусе проекта;
- получают лейблы `projects.deckhouse.io/project` и `projects.deckhouse.io/project-template`, но **не получают** лейбл `heritage: multitenancy-manager`;
- получают аннотацию `helm.sh/resource-policy: keep`, поэтому релиз проекта их не удаляет.

{% alert level="warning" %}
После создания объект с лейблом `unmanaged` не зависит от шаблона и должен управляться вручную.
{% endalert %}

## Реализация валидации изменений объектов с помощью пользовательского лейбла

Модуль `multitenancy-manager` использует ValidatingAdmissionPolicy для защиты ресурсов с лейблом `heritage: multitenancy-manager` от ручных изменений.
Вы можете реализовать аналогичную валидацию для ресурсов с любым лейблом.

### Принцип работы валидации в multitenancy-manager

`multitenancy-manager` валидирует объекты с лейблом `heritage: multitenancy-manager`.
Для этого используются следующие ресурсы:

1. ValidatingAdmissionPolicy — определяет правила валидации:
   - Операции: `CREATE`, `UPDATE` и `DELETE`;
   - Исключения: запросы компонентов кластера не проверяются, то есть запросы системных групп (`system:nodes`, сервисные аккаунты `kube-system` и `d8-system`) и системных пользователей (`system:apiserver`, `system:kube-controller-manager`, `system:kube-scheduler` и другие);
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
           operations:  ["CREATE", "UPDATE", "DELETE"]
           resources:   ["*"]
           scope: "*"
     matchConditions:
       # Пропускает компоненты кластера, как и политика модуля.
       - name: exclude-system-groups
         expression: '!(["system:nodes", "system:serviceaccounts:kube-system", "system:serviceaccounts:d8-system"].exists(e, (e in request.userInfo.groups)))'
       - name: exclude-system-users
         expression: '!(["system:apiserver", "system:kube-controller-manager", "system:kube-scheduler"].exists(e, (e == request.userInfo.username)))'
     variables:
       # В запросе DELETE нет object, есть только oldObject.
       - name: subject
         expression: 'object == null ? oldObject : object'
     validations:
       - expression: 'request.userInfo.username == "system:serviceaccount:my-namespace:my-service-account"' # Замените на ваш ServiceAccount.
         reason: Forbidden
         messageExpression: 'variables.subject.kind == ''Namespace'' ? ''This resource is managed by '' + variables.subject.metadata.name + '' system. Manual modification is forbidden.''
           : ''This resource is managed by '' + variables.subject.metadata.namespace + '' system. Manual modification is forbidden.'''
   ---
   apiVersion: admissionregistration.k8s.io/v1
   kind: ValidatingAdmissionPolicyBinding
   metadata:
     name: my-custom-label-validation
   spec:
     policyName: my-custom-label-validation
     validationActions: [Deny, Audit]
     matchResources:
       objectSelector:
         matchLabels:
           heritage: my-custom-label
   ```

1. Настройте параметры валидации:

   - `policyName` — уникальное имя политики (должно совпадать с `Policy` и `Binding`);
   - `request.userInfo.username` — имя ServiceAccount, которому разрешено изменять объекты (замените на ваш ServiceAccount);
   - `heritage: my-custom-label` — значение лейбла `heritage` для ваших ресурсов (замените на ваше значение). Значения `multitenancy-manager` и `deckhouse` использовать запрещено;
   - `failurePolicy` — определяет поведение при ошибке вычисления политики, например при ошибке в выражении: `Fail` — отклонить запрос, `Ignore` — пропустить его. Проверка, которая вернула `false`, при действии `Deny` отклоняет запрос в любом случае;
   - `matchConditions` — запросы, которые политика не проверяет. Оставьте в этом списке компоненты кластера, иначе политика заблокирует удаление неймспейсов и сборку мусора для объектов с лейблом;
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

Модуль `multitenancy-manager` позволяет администраторам кластера определять для каждого [проекта](../../glossary.html#project), какие
cluster-wide-ресурсы (например, StorageClass, ClusterIssuer, ClusterRole, LoadBalancerClass) можно
использовать из неймспейсов проектов, и какое значение используется по умолчанию.

Механизм работает независимо от RBAC. RBAC определяет, *кто может создавать и изменять* объекты, а механизм управления доступом к cluster-wide-ресурсам — *какие ресурсы* могут использовать эти объекты.

В механизме участвуют четыре кастомных ресурса:

- [GrantableClusterResourceDefinition](/modules/multitenancy-manager/cr.html#grantableclusterresourcedefinition) — регистрирует тип cluster-wide-ресурсов, доступом к которому можно управлять. Такие ресурсы поставляются DP или разработчиками модулей;
- [GrantableClusterResourceReference](/modules/multitenancy-manager/cr.html#grantableclusterresourcereference) — определяет, где используется зарегистрированный cluster-wide-ресурс. Например, какое поле ресурса содержит ссылку на него. Такие ресурсы поставляются модулями;
- [ClusterResourceGrantPolicy](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy) — задаёт правила доступа: разрешённые и запрещённые ресурсы, а также ресурс, используемый по умолчанию. Неймспейсы, к которым применяется политика, администратор кластера выбирает двумя селекторами: `projectSelector` сопоставляется с лейблами объекта Project, а `namespaceSelector` — с лейблами объекта Namespace. Политика без селекторов — библиотечная: она применяется к проекту только через `grantPolicies` его ProjectTemplate;
- [AvailableClusterResource](/modules/multitenancy-manager/cr.html#availableclusterresource) — список cluster-wide-ресурсов, доступных проекту по применимым политикам, который предназначен только для чтения. Контроллер поддерживает его в каждом неймспейсе каждого проекта для каждого GrantableClusterResourceDefinition.

Пока администратор не создал ClusterResourceGrantPolicy, доступность ресурсов определяется их регистрацией: ресурсы доступны всем проектам, если в GrantableClusterResourceDefinition задано `defaultAvailability: All` (значение по умолчанию) и ресурс не попадает под фильтры `excluded`.
Проверка доступа выполняется только для объектов в неймспейсах проектов. Если политика доступа изменяется, уже используемые существующими объектами cluster-wide-ресурсы остаются доступными для этих объектов.

Пока контроллер модуля недоступен, вебхуки отклоняют запросы. До его восстановления объекты зарегистрированных типов (например, PersistentVolumeClaim, Service или RoleBinding) в неймспейсах проектов могут создавать и изменять только компоненты платформы и участники группы `system:masters`. Чтобы контроллер оставался доступным, включите [`highAvailability`](/modules/multitenancy-manager/configuration.html#parameters-highavailability).

Подробное описание механизма управления доступом приведено [в документации модуля `multitenancy-manager`](/modules/multitenancy-manager/#управление-доступом-к-cluster-wide-ресурсам).
