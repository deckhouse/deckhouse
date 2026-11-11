---
title: Настройка доступа к пакетам
permalink: ru/admin/configuration/marketplace/package-access.html
description: "Выбор пакетов приложений, которые пользователи проектов могут устанавливать из Deckhouse Platform Marketplace, с помощью ClusterResourceGrantPolicy."
lang: ru
search: ClusterResourceGrantPolicy, applicationpackages, ApplicationPackage, доступ к пакетам, пакеты в проекте, multitenancy-manager
---

По умолчанию пользователь, которому разрешено создавать объекты Application в неймспейсе, может устанавливать приложения из любых пакетов. Если включён модуль [`multitenancy-manager`](/modules/multitenancy-manager/), администратор кластера может ограничить набор пакетов, доступных для установки в каждом [проекте](../../../glossary.html#project) и каждом неймспейсе проекта. Запрос на создание Application с недоступным пакетом будет отклонён.

Для управления доступом к пакетам Deckhouse Platform регистрирует ApplicationPackage как cluster-wide-ресурс с именем `applicationpackages`. Модуль `multitenancy-manager` проверяет значение [поля `spec.packageName`](../../../reference/api/cr.html#application-v1alpha1-spec-packagename) объектов Application в неймспейсах проектов. Доступ к пакетам настраивается с помощью [ресурса ClusterResourceGrantPolicy](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy). При этом политика не предоставляет права RBAC: для установки приложения пользователю по-прежнему нужны права на создание объектов Application в соответствующем неймспейсе.

Подробнее о механизме управления доступом — [в документации модуля `multitenancy-manager`](/modules/multitenancy-manager/#управление-доступом-к-cluster-wide-ресурсам).

## Разрешение пакетов в проекте

Чтобы разрешить в проекте `my-project` установку только пакетов `postgres` и `redis`, создайте следующую политику ClusterResourceGrantPolicy:

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha2
kind: ClusterResourceGrantPolicy
metadata:
  name: my-project-packages
spec:
  projectSelector:
    matchLabels:
      projects.deckhouse.io/project: my-project
  resources:
    - resourceName: applicationpackages
      allowed:
        - postgres
        - redis
```

В поле `allowed` указываются имена объектов ApplicationPackage, соответствующие значениям [поля `spec.packageName`](../../../reference/api/cr.html#application-v1alpha1-spec-packagename) объекта Application. Чтобы получить список доступных пакетов, выполните команду `d8 k get ap`.

Модуль `multitenancy-manager` автоматически добавляет каждому объекту Project лейбл `projects.deckhouse.io/project: <PROJECT_NAME>`, в результате чего политика применяется ко всем неймспейсам выбранного проекта, включая дополнительные неймспейсы, созданные после применения политики.

Чтобы применить политику к нескольким проектам, используйте один из лейблов:

- лейбл `projects.deckhouse.io/project-template: <PROJECT_TEMPLATE_NAME>`, который выбирает все проекты, созданные по одному шаблону;
- собственный лейбл на объектах Project, например `team: payments`, чтобы выбрать произвольную группу проектов.

Если к проекту применяется политика с полем `allowed`, пакеты, которые не перечислены в нём, становятся недоступны к установке. Если к проекту применяется несколько политик, их списки `allowed` объединяются.

## Разрешение пакетов в одном неймспейсе проекта

С помощью [параметра `namespaceSelector`](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy-v1alpha2-spec-namespaceselector) можно ограничить действие политики отдельными неймспейсами проекта. Политика применяется только к неймспейсам, лейблы которых соответствуют селектору. На остальные неймспейсы проекта такая политика не распространяется. Если их не ограничивает другая политика, в них доступны все пакеты.

Например, чтобы разрешить установку пакета `postgres` только в неймспейсе `my-project-db` проекта `my-project`, а в остальных неймспейсах проекта запретить установку любых пакетов, создайте две политики:

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha2
kind: ClusterResourceGrantPolicy
metadata:
  name: my-project-no-packages
spec:
  projectSelector:
    matchLabels:
      projects.deckhouse.io/project: my-project
  resources:
    - resourceName: applicationpackages
      availabilityDefault: None
---
apiVersion: multitenancy.deckhouse.io/v1alpha2
kind: ClusterResourceGrantPolicy
metadata:
  name: my-project-db-packages
spec:
  projectSelector:
    matchLabels:
      projects.deckhouse.io/project: my-project
  namespaceSelector:
    matchLabels:
      kubernetes.io/metadata.name: my-project-db
  resources:
    - resourceName: applicationpackages
      allowed:
        - postgres
```

Первая политика (`my-project-no-packages`) делает все пакеты недоступными для установки во всех неймспейсах проекта. Вторая (`my-project-db-packages`) разрешает пакет `postgres` в неймспейсе `my-project-db`. Если пакет разрешён хотя бы одной политикой, он доступен для установки, даже если другая политика задаёт [`availabilityDefault: None`](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-availabilitydefault).

Условие `NewNamespacesCovered` в статусе второй политики имеет значение `False`, потому что новый дополнительный неймспейс проекта не соответствует её `namespaceSelector`. Однако такие неймспейсы охватывает первая политика, поэтому установка пакетов в них запрещена.

## Запрет отдельных пакетов

Чтобы запретить установку отдельных пакетов, сохранив доступ к остальным, перечислите их имена в поле [`denied`](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-denied).

Пример политики, которая запрещает установку пакета `experimental-db` во всех проектах:

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha2
kind: ClusterResourceGrantPolicy
metadata:
  name: deny-experimental-db
spec:
  projectSelector: {}
  resources:
    - resourceName: applicationpackages
      denied:
        - experimental-db
```

Пустой [параметр `projectSelector`](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy-v1alpha2-spec-projectselector) выбирает все проекты. Пакет, указанный в `denied`, недоступен для установки, даже если он разрешён другой политикой.

## Пакеты в шаблоне проекта и в проекте

Правила доступа к пакетам можно задать [в поле `spec.resources`](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha2-spec-resources) шаблона проекта, чтобы они действовали во всех проектах, созданных по этому шаблону, или в поле [`spec.grants.resources`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-grants) объекта Project для отдельного проекта.

Например, чтобы разрешить в проектах шаблона установку только пакетов `postgres` и `redis`, добавьте в шаблон следующую запись:

```yaml
spec:
  resources:
    - resourceName: applicationpackages
      allowed:
        - postgres
        - redis
```

Подробнее — в разделах [«Выдача cluster-wide-ресурсов через шаблон проекта»](/modules/multitenancy-manager/usage.html#выдача-cluster-wide-ресурсов-через-шаблон-проекта) и [«Выдача cluster-wide-ресурсов одному проекту»](/modules/multitenancy-manager/usage.html#выдача-cluster-wide-ресурсов-одному-проекту).

## Проверка доступных пакетов

Пакеты, доступные для установки в неймспейсе, перечислены [в объекте AvailableClusterResource](/modules/multitenancy-manager/cr.html#availableclusterresource) под именем `applicationpackages` в соответствующем неймспейсе. Этот же список доступен пользователям проекта.

Чтобы просмотреть список, выполните следующую команду:

```shell
d8 k get available applicationpackages -n <NAMESPACE> -o yaml
```

Запросы от компонентов платформы и участников группы `system:masters` не проходят проверку политик. Для проверки ограничений используйте учётную запись пользователя проекта.

Например, чтобы выполнить серверный пробный запуск (dry run) манифеста Application без создания объекта, выполните следующую команду:

```shell
d8 k apply -f application.yaml --dry-run=server --as <USER_NAME>
```

## Приложения, установленные до введения ограничений

Введение ограничений из политики не приводит к удалению ранее установленных приложений. При изменении существующего объекта Application доступность его пакета проверяется только в том случае, если меняется сам пакет. Поэтому приложение, установленное до того, как его пакет стал недоступен в проекте, продолжает работать.

Такие приложения учитывает метрика `d8_cluster_objects_grant_violated`, и срабатывает [алерт `ClusterResourceGrantPolicyViolation`](../../../reference/alerts.html#multitenancy-manager-clusterresourcegrantpolicyviolation). Алерт перестаёт срабатывать, когда пакет снова становится доступен в проекте или при удалении соответствующего объекта Application.

## Ограничения

При настройке доступа к пакетам учитывайте следующие особенности и ограничения:

- Политики действуют только в неймспейсах проектов. На неймспейсы виртуальных проектов `default` и `deckhouse` они не распространяются, поэтому там можно устанавливать приложения из любых пакетов.
- Политика выбирает доступные пакеты по имени. Она не ограничивает репозиторий, из которого устанавливается доступный пакет (`spec.packageRepositoryName`), и его версию.
- Политика не скрывает недоступные пакеты. Любой аутентифицированный пользователь может читать объекты ApplicationPackageVersion всех пакетов, в том числе недоступных в его проекте. В поле `status.usedBy` версии перечислены приложения, установленные из неё, и их неймспейсы во всех проектах.
- При недоступности контроллера модуля `multitenancy-manager` объекты Application в неймспейсах проектов нельзя создать или изменить. Чтобы контроллер оставался доступным, включите [параметр `highAvailability`](/modules/multitenancy-manager/configuration.html#parameters-highavailability) в настройках модуля `multitenancy-manager`.
