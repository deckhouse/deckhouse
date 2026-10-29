---
title: Мультитенантность
permalink: ru/architecture/iam/multitenancy.html
lang: ru
search: мультитенантность, ProjectTemplate, Project, изоляция проекта
description: Как устроена мультитенантность в Deckhouse Platform.
---

Модуль [`multitenancy-manager`](/modules/multitenancy-manager/) позволяет создавать изолированные [проекты](../../glossary.html#project) в Deckhouse Platform (DP). Проекты обеспечивают квоты ресурсов, сетевую изоляцию и функции безопасности, выходящие за рамки стандартных неймспейсов.

Подробнее с настройками модуля и примерами его использования можно ознакомиться в [соответствующем разделе документации](/modules/multitenancy-manager/).

## Внутренняя логика работы

### Создание проекта

Для создания проекта используются следующие кастомные ресурсы:

* [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate) — описывает шаблон проекта. Задается список ресурсов, которые будут созданы в проекте, а также схема параметров, которые можно передать при создании проекта;
* [Project](/modules/multitenancy-manager/cr.html#project) — описывает конкретный проект.

При создании Project из определенного ProjectTemplate происходит следующее:

1. Переданные [параметры](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-parameters) валидируются по OpenAPI-спецификации (параметр [`parametersSchema.openAPIV3Schema`](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha2-spec-parametersschema-openapiv3schema) ресурса [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate)).
1. Создаётся неймспейс с тем же именем, что у [Project](/modules/multitenancy-manager/cr.html#project).
1. Из шаблона создаются объекты проекта. В шаблоне `deckhouse.io/v1alpha2` настройки, например профиль безопасности подов, сетевая изоляция, сбор логов и размещение на узлах, описываются структурированными полями, и контроллер сам создаёт из них нужные объекты в каждом неймспейсе проекта. Другие объекты описываются в `manifests` и применяются с правами роли `d8:project:admin` в неймспейсах проекта, а не с правами контроллера.
   Шаблон с Helm-текстом в поле [`resourcesTemplate`](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha2-spec-resourcestemplate) рендерится через Helm. Эта форма устарела, ни один шаблон не может получить новое поле  `resourcesTemplate`, а шаблон создаётся только через `deckhouse.io/v1alpha2`.
1. Стандартные поля проекта применяются независимо от шаблона: [`.spec.quota`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-quota) преобразуется в ResourceQuota, а [`.spec.administrators`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-administrators) — в автоматически управляемый [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding).

{% alert level="warning" %}
При изменении шаблона проекта все созданные проекты будут обновлены в соответствии с новым шаблоном.
{% endalert %}

### Изоляция проекта

Неймспейс сам по себе не изолирует проект. Проект добавляет следующее:

* доступ ко всему проекту через [`.spec.administrators`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-administrators), ProjectRoleBinding и ClusterProjectRoleBinding; обычный RoleBinding в одном неймспейсе допускается для делегируемых ролей;
* ограничение ресурсов через [`.spec.quota`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-quota), которое применяется как ResourceQuota `d8-project-quota` в основном неймспейсе;
* сетевую изоляцию через поле [`networkPolicy.mode: Isolated`](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha2-spec-networkpolicy-mode) шаблона;
* профиль безопасности подов через поле [`podSecurityStandard`](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha2-spec-podsecuritystandard) шаблона.

### Управление доступом к cluster-wide-ресурсам

Объекты в неймспейсах проектов могут ссылаться на cluster-wide-ресурсы. Например, PersistentVolumeClaim может использовать StorageClass,
Certificate — ClusterIssuer, RoleBinding — ClusterRole. Модуль позволяет администраторам
кластера определять, какие cluster-wide-ресурсы можно использовать из неймспейсов
проектов, и какое значение используется по умолчанию.

Механизм работает независимо от RBAC. RBAC определяет, *кто может создавать и изменять* объекты, а механизм управления доступом к cluster-wide-ресурсам — *какие ресурсы* могут использовать эти объекты.

Механизм использует четыре кастомных ресурса:

* [GrantableClusterResourceDefinition](/modules/multitenancy-manager/cr.html#grantableclusterresourcedefinition) — регистрирует тип cluster-wide-ресурсов, доступом к которому можно управлять. Такие ресурсы поставляются DP или разработчиками модулей;
* [GrantableClusterResourceReference](/modules/multitenancy-manager/cr.html#grantableclusterresourcereference) — определяет, где используется зарегистрированный cluster-wide-ресурс. Например, какое поле ресурса содержит ссылку на него. Такие ресурсы поставляются модулями;
* [ClusterResourceGrantPolicy](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy) — задаёт правила доступа. В версии `v1alpha2` администратор кластера выбирает проекты, на которые распространяется политика, по лейблам объектов Project (`projectSelector`) и сужает выбор по лейблам неймспейсов проектов (`namespaceSelector`). Политика также определяет разрешённые и запрещённые ресурсы, а также ресурс, используемый по умолчанию;
* В каждом неймспейсе каждого проекта контроллер поддерживает [AvailableClusterResource](/modules/multitenancy-manager/cr.html#availableclusterresource) для каждого GrantableClusterResourceDefinition. Он содержит список cluster-wide-ресурсов, доступных проекту по применимым политикам, и предназначен только для чтения.

Пока администратор не создал ClusterResourceGrantPolicy, доступность ресурсов определяется их регистрацией: ресурсы доступны всем проектам, если в GrantableClusterResourceDefinition задано `defaultAvailability: All` (значение по умолчанию) и ресурс не попадает под фильтры `excluded`.
Проверка доступа выполняется только для объектов в неймспейсах проектов. Если политика доступа изменяется, уже используемые существующими объектами cluster-wide-ресурсы остаются доступными для этих объектов.

Пока контроллер модуля недоступен, вебхуки отклоняют запросы. До его восстановления объекты зарегистрированных типов (например, PersistentVolumeClaim, Service или RoleBinding) в неймспейсах проектов могут создавать и изменять только компоненты платформы и участники группы `system:masters`. Чтобы контроллер оставался доступным, включите [`highAvailability`](/modules/multitenancy-manager/configuration.html#parameters-highavailability).

Подробное описание механизма управления доступом приведено [в документации модуля `multitenancy-manager`](/modules/multitenancy-manager/#управление-доступом-к-cluster-wide-ресурсам).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`multitenancy-manager`](/modules/multitenancy-manager/) на уровне 2 модели C4 и его взаимодействия с другими компонентами DP изображены на следующей диаграмме.

![Архитектура модуля multitenancy-manager](../../images/architecture/iam/c4-l2-multitenancy-manager.ru.svg)

## Компоненты модуля

Модуль состоит из следующих компонентов:

- **Multitenancy-manager** — компонент состоит из одного контейнера **multitenancy-manager** и обеспечивает следующие функции:

  - управление кастомными ресурсами Project и ProjectTemplate;
  - валидация кастомных ресурсов Project и ProjectTemplate;
  - создание ресурсов, указанных в кастомном ресурсе ProjectTemplate, на основе параметров, заданных в Project;
  - управление механизмом грантов для кластерных ресурсов при помощи следующих кастомных ресурсов:
    [`GrantableClusterResourceDefinition`](/modules/multitenancy-manager/cr.html#grantableclusterresourcedefinition),
    [`GrantableClusterResourceReference`](/modules/multitenancy-manager/cr.html#grantableclusterresourcereference),
    [`ClusterResourceGrantPolicy`](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy) и
    [`AvailableClusterResource`](/modules/multitenancy-manager/cr.html#availableclusterresource). Формирует
    каталоги `AvailableClusterResource` для [проектов](../../glossary.html#project) и запускает валидирующие/мутирующие admission-вебхуки
    (`/is-granted`, `/defaults`, `/protect`), которые контролируют, на какие кластерные ресурсы проект может
    ссылаться, и подставляют значения по умолчанию проекта при CREATE.

   > **Внимание.** Multitenancy-manager имеет права `cluster-admin`. Объекты, описанные в поле `manifests` ресурса ProjectTemplate, применяются не с этими правами, а с правами роли `d8:project:admin` в неймспейсах проекта.

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

- **Kube-apiserver**:
  - управление кастомными ресурсами Project и ProjectTemplate;
  - валидация кастомных ресурсов Project и ProjectTemplate;
  - создание ресурсов, указанных в кастомном ресурсе ProjectTemplate, на основе параметров, заданных в Project.
  - управление кастомными ресурсами GrantableClusterResourceDefinition, GrantableClusterResourceReference, ClusterResourceGrantPolicy и AvailableClusterResource.
  - валидация кастомных ресурсов AvailableClusterResource.
  - создание validation admission-вебхуков на основе условий, заданных в кастомных ресурсах GrantableClusterResourceDefinition и GrantableClusterResourceReference.
