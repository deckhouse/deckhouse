---
title: "Модуль multitenancy-manager"
search: multitenancy
description: Создание изолированных проектов для команд разработки по шаблонам проектов.
---

Модуль позволяет создавать проекты в кластере Kubernetes. **Проект** — это изолированное окружение, в котором можно развернуть приложения.

## Основные возможности

- Создание проектов по шаблонам проектов. Шаблон задаёт профиль безопасности подов, сетевую изоляцию, сбор логов, размещение на узлах и другие политики неймспейсов проекта.
- Несколько неймспейсов в одном проекте. Настройки шаблона и выданный проекту доступ распространяются на все из них.
- Доступ ко всему проекту через `.spec.administrators`, ProjectRoleBinding и ClusterProjectRoleBinding.
- Квота ресурсов для каждого проекта.
- Проект для каждого неймспейса: неймспейс, созданный напрямую, автоматически становится проектом.
- Управление тем, какие cluster-wide-ресурсы, например StorageClass или ClusterIssuer, могут использовать проекты.

{% alert level="warning" %}
Модуль [secret-copier](/modules/secret-copier/) нельзя использовать вместе с модулем `multitenancy-manager`: он копирует секреты во все неймспейсы, в том числе в неймспейсы проектов.
{% endalert %}

## Состав проекта

В проект входят:

* **Основной неймспейс** — создаётся автоматически, его имя совпадает с именем проекта.
* **Дополнительные неймспейсы** (опционально) — создаются ресурсом [ProjectNamespace](./cr.html#projectnamespace) и получают имена вида `<имя проекта>-<имя>`. Настройки проекта (политики безопасности, сетевые политики, сбор логов и т. п.) и выданный на проект доступ автоматически распространяются на все его неймспейсы.
* **Стандартные поля** — список администраторов ([`.spec.administrators`](./cr.html#project-v1alpha3-spec-administrators)) и квоты ресурсов ([`.spec.quota`](./cr.html#project-v1alpha3-spec-quota)), управляемые самим ресурсом [Project](./cr.html#project) независимо от шаблона.
* **Ресурсы из шаблона** (опционально) — политики и настройки, описанные в [ProjectTemplate](./cr.html#projecttemplate) и создаваемые контроллером в неймспейсах проекта, а также объекты из его `manifests`, которые применяются с правами администратора проекта.

## Как работает модуль

В этом разделе описано, как модуль создаёт проекты, добавляет в них неймспейсы, выдаёт доступ и превращает существующие неймспейсы в проекты.

### Создание проекта

Для создания проекта используются [ресурсы](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/):

* [ProjectTemplate](./cr.html#projecttemplate) — ресурс, который описывает шаблон проекта: какие настройки и политики получат его неймспейсы и какие параметры можно передать при создании проекта;
* [Project](./cr.html#project) — ресурс, который описывает конкретный проект.

При создании ресурса [Project](./cr.html#project) из определённого [ProjectTemplate](./cr.html#projecttemplate) происходит следующее:

1. Переданные [параметры](./cr.html#project-v1alpha3-spec-parameters) валидируются по OpenAPI-спецификации (параметр [`parametersSchema.openAPIV3Schema`](./cr.html#projecttemplate-v1alpha2-spec-parametersschema-openapiv3schema) ресурса [ProjectTemplate](./cr.html#projecttemplate));
1. Создаётся неймспейс с тем же именем, что у [Project](./cr.html#project);
1. Из шаблона создаются ресурсы проекта:
   * для современных шаблонов (`deckhouse.io/v1alpha2`) настройки, например профиль безопасности подов, сетевая изоляция, сбор логов и размещение на узлах, описываются [структурированными полями](usage.html#структурированные-шаблоны). Контроллер сам создаёт из них нужные объекты в каждом неймспейсе проекта. Другие объекты описываются в [`manifests`](usage.html#описание-других-объектов-в-manifests) и применяются с правами администратора проекта;
   * шаблон с Helm-полем `resourcesTemplate` рендерится через Helm. Эта форма устарела, как и версия `deckhouse.io/v1alpha1` ресурса ProjectTemplate, в которой другой формы нет ([подробнее](usage.html#проверки-шаблонов)). Ни один шаблон не может получить новое `resourcesTemplate`, шаблон создаётся только через `deckhouse.io/v1alpha2`, а в одном из следующих релизов поле будет удалено, поэтому [переведите такой шаблон](usage.html#перевод-шаблона-с-helm-текста) на структурированные поля и `manifests`;
1. Стандартные поля проекта применяются независимо от шаблона: [`.spec.quota`](./cr.html#project-v1alpha3-spec-quota) преобразуется в `ResourceQuota`, а [`.spec.administrators`](./cr.html#project-v1alpha3-spec-administrators) — в автоматически управляемый [ProjectRoleBinding](./cr.html#projectrolebinding).

API ресурса Project обслуживается как `deckhouse.io/v1alpha3`. Манифесты `v1alpha2` продолжают работать благодаря вебхуку конвертации, а версия `v1alpha1` больше не обслуживается (подробнее — [в разделе «Версии API ресурса Project»](usage.html#версии-api-ресурса-project)). Поле `projectTemplateName` необязательно: если его не указать, используется шаблон `simple`, создающий только неймспейс проекта.

При создании проекта проверяется его имя. Недопустимы имена длиннее 61 символа, имена с системными префиксами `d8-` и `kube-`, имена существующих неймспейсов и неймспейсов, которые уже занял ресурс ProjectNamespace, а также имена неймспейсов, чьи ServiceAccount всё ещё указаны в кластерной привязке или в привязке другого проекта. Проект `foo-bar` рядом с проектом `foo` создать можно, но это отдельный проект, а не дополнительный неймспейс проекта `foo`. Вебхук предупреждает об этом, а такие пары показывает алерт ([подробнее](usage.html#правила-именования-проектов)).

{% alert level="warning" %}
Изменение шаблона проекта применяется ко всем проектам, созданным по нему.
{% endalert %}

### Несколько неймспейсов в проекте

Проект может включать несколько неймспейсов. Дополнительные неймспейсы создаются ресурсом [ProjectNamespace](./cr.html#projectnamespace) в основном неймспейсе проекта и получают имена вида `<имя проекта>-<имя>` (например, `backend-cache` для проекта `backend`).

На дополнительные неймспейсы автоматически распространяются:

* настройки шаблона проекта — профиль безопасности подов, сетевые политики, сбор логов и другие политики создаются в каждом неймспейсе проекта;
* доступ, выданный через [ProjectRoleBinding](./cr.html#projectrolebinding) и [ClusterProjectRoleBinding](./cr.html#clusterprojectrolebinding), включая автоматический доступ администраторов проекта.

Квота проекта ([`.spec.quota`](./cr.html#project-v1alpha3-spec-quota)) применяется в основном неймспейсе.

При удалении проекта удаляются все его неймспейсы — и основной, и дополнительные. Подробнее — [в примерах использования](usage.html#дополнительные-неймспейсы-проекта).

### Доступ к проекту

Доступ пользователей к проекту управляется на уровне всего проекта, а не отдельных неймспейсов:

* Субъекты из [`.spec.administrators`](./cr.html#project-v1alpha3-spec-administrators) автоматически получают роль администратора проекта (`d8:project:admin`).
* Ресурс [ProjectRoleBinding](./cr.html#projectrolebinding) выдаёт роль во всех неймспейсах одного проекта.
* [ClusterProjectRoleBinding](./cr.html#clusterprojectrolebinding) выдаёт роль сразу во всех проектах кластера, кроме виртуальных.

ProjectRoleBinding и ClusterProjectRoleBinding могут выдавать только ClusterRole с именами `d8:project:*`, `d8:namespace:*`, `d8:project-capability:*`, `d8:namespace-capability:*` или `d8:custom:*`. Подробнее о ролях — [в документации модуля `user-authz`](/modules/user-authz/). Подробнее о привязках — [в примерах использования](usage.html#предоставление-доступа-внутри-проекта).

### Автоматическое создание проектов для неймспейсов

Пользователи по-прежнему могут создавать неймспейсы напрямую (`d8 k create namespace`), если параметр [`allowNamespacesWithoutProjects`](configuration.html#parameters-allownamespaceswithoutprojects) не равен `false`. При `false` пользователи создают окружения как объекты Project, а напрямую неймспейсы создают только администраторы кластера (`system:masters`, `system:sudouser`), Deckhouse, контроллер модуля и системные компоненты. Каждый неймспейс, не входящий ни в один проект, становится отдельным полноценным проектом с тем же именем:

* шаблон подбирается по тому, что на неймспейсе уже есть: `secure` — если включено сканирование уязвимостей, `default` — если задан Pod Security Standard или расширенный мониторинг, иначе `simple` (только неймспейс);
* параметры проекта заполняются из текущего состояния неймспейса, поэтому при переходе внутри него ничего не меняется;
* дальше источником истины становится проект, и он редактируется как любой другой. Удалить неймспейс напрямую нельзя: чтобы убрать окружение, удалите Project, и его неймспейс удалится вместе с ним.

Системные неймспейсы (`d8-*`, `kube-*`, `upmeter-*`, `default` и всё с лейблом `heritage: deckhouse` или `heritage: upmeter`) в проекты не превращаются: их учитывает виртуальный проект `deckhouse` (кроме `default`). Лейбла, который оставляет пользовательский неймспейс без проекта, нет. Неймспейс длиннее 61 символа тоже пропускается: это лимит имени Project.

### Изоляция проекта

Неймспейс сам по себе не изолирует проект. Проект добавляет следующее:

* доступ ко всему проекту через [`.spec.administrators`](./cr.html#project-v1alpha3-spec-administrators), ProjectRoleBinding и ClusterProjectRoleBinding; обычный RoleBinding в одном неймспейсе допускается для делегируемых ролей;
* ограничение ресурсов через [`.spec.quota`](./cr.html#project-v1alpha3-spec-quota), которое применяется как ResourceQuota `d8-project-quota` в основном неймспейсе;
* сетевую изоляцию через поле [`networkPolicy.mode: Isolated`](./cr.html#projecttemplate-v1alpha2-spec-networkpolicy-mode) шаблона;
* профиль безопасности подов через поле [`podSecurityStandard`](./cr.html#projecttemplate-v1alpha2-spec-podsecuritystandard) шаблона.

## Управление доступом к cluster-wide-ресурсам

Объекты в неймспейсах проектов могут ссылаться на cluster-wide-ресурсы. Например, PersistentVolumeClaim может использовать StorageClass,
Certificate — ClusterIssuer, RoleBinding — ClusterRole. Модуль позволяет администраторам
кластера определять, какие cluster-wide-ресурсы можно использовать из неймспейсов
проектов, и какое значение используется по умолчанию.

Механизм работает независимо от RBAC. RBAC определяет, *кто может создавать и изменять* объекты, а механизм управления доступом к cluster-wide-ресурсам — *какие ресурсы* могут использовать эти объекты.

### Принцип работы управления доступом

Для управления доступом к cluster-wide-ресурсам используются следующие ресурсы:

* [GrantableClusterResourceDefinition](./cr.html#grantableclusterresourcedefinition) регистрирует тип cluster-wide-ресурсов, доступом к которому можно управлять. Такие ресурсы поставляются Deckhouse Platform или разработчиками модулей.
* [GrantableClusterResourceReference](./cr.html#grantableclusterresourcereference) определяет, где используется зарегистрированный cluster-wide-ресурс. Например, какое поле ресурса содержит ссылку на него. Такие ресурсы поставляются модулями.
* [ClusterResourceGrantPolicy](./cr.html#clusterresourcegrantpolicy) задаёт правила доступа. Администратор кластера с помощью лейблов выбирает проекты, на которые распространяется политика, определяет разрешённые и запрещённые ресурсы, а также ресурс, используемый по умолчанию.
* В каждом неймспейсе каждого проекта контроллер поддерживает [AvailableClusterResource](./cr.html#availableclusterresource) для каждого GrantableClusterResourceDefinition. Он содержит список cluster-wide-ресурсов, доступных проекту по применимым политикам, и предназначен только для чтения.
* При создании или изменении объекта вебхуки проверяют, разрешено ли ему использовать указанный cluster-wide-ресурс. При создании объекта также может автоматически подставляться ресурс, заданный для проекта по умолчанию.

Пока контроллер модуля недоступен, вебхуки отклоняют запросы. До его восстановления объекты зарегистрированных типов (например, PersistentVolumeClaim, Service или RoleBinding) в неймспейсах проектов могут создавать и изменять только компоненты платформы и участники группы `system:masters`. Чтобы контроллер оставался доступным, включите [`highAvailability`](configuration.html#parameters-highavailability).

<script src="/assets/js/mermaid.min.js"></script>
<script>mermaid.initialize({ startOnLoad: true });</script>

<pre class="mermaid">
flowchart LR
    A["Разработчик модуля или Deckhouse Platform<br/>поставляет<br/>GrantableClusterResourceDefinition<br/>и GrantableClusterResourceReference"] --> C
    B["Администратор кластера<br/>создаёт<br/>ClusterResourceGrantPolicy"] --> C["Контроллер"]
    C --> D["Создаёт<br/>AvailableClusterResource<br/>в неймспейсе каждого проекта"]
    E["Пользователь создаёт объект<br/>(например,<br/>PersistentVolumeClaim)"] --> F["Mutating-вебхук<br/>/defaults"]
    F --> G["Validating-вебхук<br/>/is-granted"]
    D -. Доступные имена .-> G
    G --> H["Объект создан<br/>или отклонён"]
</pre>

Пока администратор не создал ClusterResourceGrantPolicy, доступность ресурсов определяется их регистрацией: ресурсы доступны всем проектам, если в GrantableClusterResourceDefinition задано `defaultAvailability: All` (значение по умолчанию) и ресурс не попадает под фильтры `excluded`.

Квотирование ресурсов не является частью данного механизма — оно делегировано стандартному
ресурсу Kubernetes ResourceQuota.

Проверка доступа выполняется только для объектов в неймспейсах проектов. Если политика доступа изменяется, уже используемые существующими объектами cluster-wide-ресурсы остаются доступными для этих объектов.

### Порядок проверки доступа и подстановки значений по умолчанию

Механизм определяет доступность cluster-wide-ресурсов и значения по умолчанию на основе зарегистрированных ресурсов и применяемых к проекту политик.

#### Определение доступности cluster-wide-ресурса

Если на cluster-wide-ресурс распространяется несколько правил, его доступность определяется в следующем порядке:

1. Значение [`excluded`](./cr.html#grantableclusterresourcedefinition-v1alpha1-spec-excluded) в GrantableClusterResourceDefinition — ресурс недоступен независимо от настроек политик.
1. Значение [`denied`](./cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-denied) и [`deniedSelector`](./cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-deniedselector) в соответствующей записи ClusterResourceGrantPolicy.
1. Значение [`allowed`](./cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-allowed) и [`allowedSelector`](./cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-allowedselector) в соответствующей записи ClusterResourceGrantPolicy.
1. Значение [`availabilityDefault`](./cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-availabilitydefault) в записях ClusterResourceGrantPolicy, которые применяются к проекту. Если в записи задан `allowed` или `allowedSelector` без `availabilityDefault`, все ресурсы вне её списков недоступны. Если хотя бы одна применимая запись задаёт `All`, это значение имеет приоритет над `None` в других записях.
1. Значение [`defaultAvailability`](./cr.html#grantableclusterresourcedefinition-v1alpha1-spec-defaultavailability) в GrantableClusterResourceDefinition.

Решает первый подходящий шаг. Шаги 2–4 объединяют записи всех ClusterResourceGrantPolicy, которые применяются к проекту, включая служебные политики его шаблона.

#### Подстановка значений по умолчанию

Поведение при создании объекта зависит от режима, заданного [в GrantableClusterResourceReference](./cr.html#grantableclusterresourcereference-v1alpha1-spec-fieldpaths-defaulting):

* `None` — значение проверяется на доступность, но не подставляется автоматически;
* `FillEmpty` — если значение не указано, подставляется значение по умолчанию для проекта;
* `Coerce` — если значение не указано или указанный cluster-wide-ресурс недоступен проекту, подставляется значение по умолчанию для проекта. О замене указанного значения сообщается предупреждением (admission warning) в ответе на запрос.

Значение по умолчанию для проекта определяется в следующем порядке:

1. Значение [`default`](./cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-default) из соответствующей записи ClusterResourceGrantPolicy.
1. Значение, определённое с помощью [`defaultFrom`](./cr.html#grantableclusterresourcedefinition-v1alpha1-spec-defaultfrom) в GrantableClusterResourceDefinition.
1. Если значение не найдено, оно не подставляется.

#### Проверка существующих объектов

При изменении политики доступа значения, уже используемые существующими объектами, сохраняются. При обновлении объекта проверяются только новые значения: если значение в поле не изменилось, оно остаётся допустимым даже после ограничения доступа.

Это позволяет существующим объектам продолжать работу после ограничения доступа к cluster-wide-ресурсам.

#### Системные запросы

Запросы компонентов платформы и участников группы `system:masters` не проверяются. Компоненты платформы — это API-сервер, ServiceAccount Deckhouse, этого модуля и модуля `user-authz`, все ServiceAccount неймспейсов `d8-system`, `kube-system` и `d8-user-authz`, а также kubelet. Чтобы проверить политику, выполните запрос от имени пользователя проекта, например `d8 k --as <USER_NAME>`.

### Мониторинг нарушений политик доступа

Если существующий объект использует cluster-wide-ресурс, который стал недоступен проекту после изменения политики, объект продолжает работать, но такое расхождение отслеживается средствами мониторинга.

При обнаружении таких объектов срабатывает [алерт `ClusterResourceGrantPolicyViolation`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#multitenancy-manager-clusterresourcegrantpolicyviolation). Информацию о нарушениях можно просмотреть на дашборде Grafana в разделе «Security» → «Cluster Resource Grant Violations».

Для мониторинга используется метрика `d8_cluster_objects_grant_violated`. Её отдаёт контроллер модуля: он пересчитывает нарушения при каждой обработке неймспейса проекта и не реже чем раз в две минуты по тем же правилам доступности, что и вебхук `/is-granted`.

### Ресурсы, регистрируемые Deckhouse Platform

Deckhouse Platform регистрирует следующие cluster-wide-ресурсы:

| Имя определения | Cluster-wide-ресурс | Где используется | Режим подстановки значения по умолчанию |
| --- | --- | --- | --- |
| `storageclasses` | StorageClass (storage.k8s.io) | PersistentVolumeClaim `.spec.storageClassName` | `Coerce` |
| `loadbalancerclasses` | Значение `loadbalancerclass` | Service `.spec.loadBalancerClass` (для сервисов типа `LoadBalancer`) | `FillEmpty` |
| `clusterissuers` | ClusterIssuer (cert-manager.io) | Certificate `.spec.issuerRef.name`; Ingress: аннотация `cert-manager.io/cluster-issuer` | `FillEmpty` или `None` |
| `clusterroles` | ClusterRole (rbac.authorization.k8s.io) | RoleBinding `.roleRef.name` | `None` |

Регистрация `clusterroles` исключает все ClusterRole без лейбла `rbac.deckhouse.io/delegatable: "true"`. По умолчанию RoleBinding в проекте может ссылаться на роли `d8:namespace:*` и `d8:project:*`, устаревшие псевдонимы `d8:use:role:*` и роли `user-authz:user`, `user-authz:privileged-user`, `user-authz:editor` и `user-authz:admin` упрощённой модели.

Определение `clusterissuers` регистрируется только при включённом модуле `cert-manager`.

### Ресурсы механизма управления доступом

| Ресурс | Область | Кто создаёт | Ручное создание | Назначение |
| --- | --- | --- | --- | --- |
| [GrantableClusterResourceDefinition](./cr.html#grantableclusterresourcedefinition) | Кластер | Разработчик модуля или Deckhouse Platform | Разрешено для кастомных ресурсов | Регистрирует тип cluster-wide-ресурса, доступом к которому можно управлять |
| [GrantableClusterResourceReference](./cr.html#grantableclusterresourcereference) | Кластер | Разработчик модуля | Разрешено для полей кастомных ресурсов | Определяет, где используется зарегистрированный cluster-wide-ресурс |
| [ClusterResourceGrantPolicy](./cr.html#clusterresourcegrantpolicy) | Кластер | Администратор кластера | Обязательно | Определяет доступные и запрещённые ресурсы, а также ресурс, используемый проектом по умолчанию |
| [AvailableClusterResource](./cr.html#availableclusterresource) | Неймспейс | Контроллер (автоматически) | Запрещено (защищено вебхуком) | Каталог доступных проекту ресурсов, только для чтения |

Подробные сценарии настройки доступа, просмотр доступных ресурсов в проектах, правила проверки и подстановки значений по умолчанию, а также рекомендации для разработчиков модулей приведены [в разделе «Примеры использования»](usage.html#управление-доступом-к-cluster-wide-ресурсам).
