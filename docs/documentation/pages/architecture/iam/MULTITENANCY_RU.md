---
title: Мультитенантность
permalink: ru/architecture/iam/multitenancy.html
lang: ru
search: мультитенантность, ProjectTemplate, Project, изоляция проекта
description: Как устроена мультитенантность в Deckhouse Kubernetes Platform.
---

Модуль [`multitenancy-manager`](/modules/multitenancy-manager/) позволяет создавать изолированные проекты в Deckhouse Kubernetes Platform (DKP). Проекты обеспечивают квоты ресурсов, сетевую изоляцию и функции безопасности, выходящие за рамки стандартных неймспейсов.

Подробнее с настройками модуля и примерами его использования можно ознакомиться в [соответствующем разделе документации](/modules/multitenancy-manager/).

## Внутренняя логика работы

### Создание проекта

Для создания проекта используются следующие кастомные ресурсы:

* [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate) — описывает шаблон проекта. Задается список ресурсов, которые будут созданы в проекте, а также схема параметров, которые можно передать при создании проекта;
* [Project](/modules/multitenancy-manager/cr.html#project) — описывает конкретный проект.

При создании Project из определенного ProjectTemplate происходит следующее:

1. Переданные [параметры](/modules/multitenancy-manager/cr.html#project-v1alpha2-spec-parameters) валидируются по OpenAPI-спецификации (параметр [`openAPIV3Schema`](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha1-spec-parametersschema-openapiv3schema) ресурса [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate)).
1. Выполняется рендеринг [шаблона для ресурсов](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha1-spec-resourcestemplate) с помощью [Helm](https://helm.sh/docs/). Значения для рендеринга берутся из параметра [`parameters`](/modules/multitenancy-manager/cr.html#project-v1alpha2-spec-parameters) ресурса [Project](/modules/multitenancy-manager/cr.html#project).
1. Создаётся неймспейс с именем, которое совпадает c именем [Project](/modules/multitenancy-manager/cr.html#project).
1. По очереди создаются все ресурсы, описанные в шаблоне.

{% alert level="warning" %}
При изменении шаблона проекта все созданные проекты будут обновлены в соответствии с новым шаблоном.
{% endalert %}

### Изоляция проекта

В основе проекта используется механизм изоляции ресурсов в рамках неймспейса.
Неймспейсы позволяют группировать поды, сервисы, секреты и другие объекты, но не обеспечивают полноценной изоляции.
Проект расширяет функциональность неймспейсов, предлагая дополнительные инструменты для повышения уровня контроля и безопасности.

Для управления уровнем изоляции проекта можно использовать возможности Kubernetes, например:

* **Ресурсы контроля доступа** (AuthorizationRule / RoleBinding) — позволяют управлять взаимодействием объектов внутри неймспейса. С их помощью можно задавать правила и назначать роли, чтобы точно контролировать, кто и что может делать в проекте.
* **Ресурсы контроля использования нагрузки** (ResourceQuota) — с их помощью можно задать лимиты на использование процессорного времени (CPU), оперативной памяти (RAM), а также количества объектов внутри неймспейса. Это помогает избежать чрезмерной нагрузки и обеспечивает мониторинг за приложениями в рамках проекта.
* **Ресурсы контроля сетевой связности** (NetworkPolicy) — управляют входящим и исходящим сетевым трафиком в неймспейсе. Таким образом, можно настроить разрешенные подключения между подами, улучшить безопасность и управляемость сетевого взаимодействия в рамках проекта.

Эти инструменты можно комбинировать, чтобы настроить проект в соответствии с требованиями вашего приложения.

### Управление доступом к кластерным ресурсам

Проекты регулярно ссылаются на кластерные ресурсы: `PersistentVolumeClaim` указывает `StorageClass`,
`Certificate` — `ClusterIssuer`, `RoleBinding` — `ClusterRole`. Модуль также управляет для каждого
проекта, **какие** кластерные ресурсы можно использовать из неймспейсов проектов, и какое значение
используется по умолчанию. Это отдельный механизм от RBAC: RBAC решает, *кто может создать* объект,
гранты — *какие кластерные ресурсы этот объект может ссылать*.

Механизм использует четыре кастомных ресурса:

- [`GrantableClusterResourceDefinition`](/modules/multitenancy-manager/cr.html#grantableclusterresourcedefinition) (`gcrd`, cluster-scoped) — регистрирует кластерный ресурс как управляемый грантами (поставляется платформой и модулями).
- [`GrantableClusterResourceReference`](/modules/multitenancy-manager/cr.html#grantableclusterresourcereference) (`gcrr`, cluster-scoped) — объявляет, какое поле какого CRD валидируется/подставляется (поставляется модулями).
- [`ClusterResourceGrantPolicy`](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy) (`crgp`, cluster-scoped) — списки разрешений/запретов и дефолты администратора для проекта (единственный ручной шаг для контроля доступа).
- [`AvailableClusterResource`](/modules/multitenancy-manager/cr.html#availableclusterresource) (`available`, namespaced, read-only) — формируемый контроллером каталог, который проект читает, чтобы узнать доступные ресурсы.

Пока администратор не создал `ClusterResourceGrantPolicy`, все ресурсы доступны (разрешающий дефолт).
Валидация применяется только к неймспейсам проектов; существующие объекты при UPDATE сохраняют
значения (grandfathering).

Полное руководство — в [руководстве по использованию модуля](/modules/multitenancy-manager/usage_ru.html#управление-доступом-к-кластерным-ресурсам-гранты).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`multitenancy-manager`](/modules/multitenancy-manager/) на уровне 2 модели C4 и его взаимодействия с другими компонентами DKP изображены на следующей диаграмме.

![Архитектура модуля multitenancy-manager](../../images/architecture/iam/c4-l2-multitenancy-manager.ru.png)

## Компоненты модуля

Модуль состоит из следующих компонентов:

- **Multitenancy-manager** — компонент состоит из одного контейнера **multitenancy-manager** и обеспечивает следующие функции:

  - управление кастомными ресурсами Project и ProjectTemplate;
  - валидация кастомных ресурсов Project и ProjectTemplate;
  - валидация стандартного ресурса Namespace если в параметрах модуля `multitenancy-manager` задано [`.spec.settings.allowNamespacesWithoutProjects=false`](/modules/multitenancy-manager/configuration.html#parameters-allownamespaceswithoutprojects);
  - создание ресурсов, указанных в кастомном ресурсе ProjectTemplate, на основе параметров, заданных в Project.

   > **Внимание.** Multitenancy-manager имеет права `cluster-admin`, что позволяет создавать любые объекты, описанные в ресурсе ProjectTemplate.

- **Cluster-objects-controller** — управляет механизмом грантов: кастомными ресурсами
  [`GrantableClusterResourceDefinition`](/modules/multitenancy-manager/cr.html#grantableclusterresourcedefinition),
  [`GrantableClusterResourceReference`](/modules/multitenancy-manager/cr.html#grantableclusterresourcereference),
  [`ClusterResourceGrantPolicy`](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy) и
  [`AvailableClusterResource`](/modules/multitenancy-manager/cr.html#availableclusterresource). Формирует
  каталоги `AvailableClusterResource` для проектов и запускает валидирующие/мутирующие admission-вебхуки
  (`/is-granted`, `/defaults`, `/protect`), которые контролируют, какие кластерные ресурсы проект может
  ссылать, и подставляют дефолты проекта при CREATE.

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

- **Kube-apiserver**:
  - управление кастомными ресурсами Project и ProjectTemplate;
  - валидация кастомных ресурсов Project, ProjectTemplate, а также стандартного ресурса Namespace;
  - создание ресурсов, указанных в кастомном ресурсе ProjectTemplate, на основе параметров, заданных в Project.
  - грант-валидация: вебхуки cluster-objects-controller валидируют и подставляют дефолты для ссылок на грантуемые кластерные ресурсы (напр. `StorageClass`, `ClusterIssuer`, `ClusterRole`) в объектах, созданных в неймспейсах проектов, и отклоняют ручное создание объектов `AvailableClusterResource`.
