---
title: Доступ к пакетам
permalink: ru/admin/configuration/marketplace/package-access.html
description: "Выбор пакетов приложений, которые пользователи проектов могут устанавливать из Deckhouse Platform Marketplace, с помощью ClusterResourceGrantPolicy."
lang: ru
search: ClusterResourceGrantPolicy, applicationpackages, ApplicationPackage, доступ к пакетам, пакеты в проекте, multitenancy-manager
---

По умолчанию пользователь, которому разрешено создавать объекты Application в неймспейсе, может установить приложение из любого пакета. Если включён модуль [`multitenancy-manager`](/modules/multitenancy-manager/), администратор кластера может выбрать пакеты, доступные в каждом [проекте](../../../glossary.html#project) и в каждом неймспейсе проекта. Application с недоступным пакетом отклоняется.

Deckhouse Platform регистрирует ApplicationPackage как cluster-wide-ресурс модуля с именем `applicationpackages`, и модуль проверяет поле `spec.packageName` объектов Application в неймспейсах проектов. Доступные пакеты выбираются с помощью [ClusterResourceGrantPolicy](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy) для ресурса `applicationpackages`. Политика не выдаёт прав RBAC: чтобы установить приложение, пользователю по-прежнему нужна роль, которая разрешает создавать объекты Application в неймспейсе. Устройство механизма описано [в документации модуля `multitenancy-manager`](/modules/multitenancy-manager/#управление-доступом-к-cluster-wide-ресурсам).

## Разрешение пакетов в проекте

Чтобы разрешить в проекте `my-project` только пакеты `postgres` и `redis`, создайте следующий ClusterResourceGrantPolicy:

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

В `allowed` указываются имена объектов ApplicationPackage — те же, что в поле `spec.packageName` объекта Application. Чтобы получить список пакетов, выполните `d8 k get ap`.

Модуль ставит на каждый Project лейбл `projects.deckhouse.io/project: <PROJECT_NAME>`, поэтому политика применяется ко всем неймспейсам `my-project`, включая дополнительные неймспейсы, созданные позже. Чтобы выбрать несколько проектов, используйте лейбл `projects.deckhouse.io/project-template: <PROJECT_TEMPLATE_NAME>`, который выбирает проекты одного шаблона, или собственный лейбл на объектах Project, например `team: payments`.

Если к проекту применяется политика с `allowed`, пакеты, которых нет в списке, в нём недоступны. Если к проекту применяется несколько политик, их списки `allowed` складываются.

## Разрешение пакетов в одном неймспейсе проекта

Политика с [`namespaceSelector`](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy-v1alpha2-spec-namespaceselector) применяется только к неймспейсам, лейблы которых соответствуют селектору. Остальные неймспейсы проекта она не ограничивает: если их не ограничивает другая политика, в них доступны все пакеты.

Чтобы разрешить пакет `postgres` только в неймспейсе `my-project-db` проекта `my-project`, а в остальных неймспейсах проекта не разрешать ни одного пакета, создайте две политики:

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

Первая политика делает все пакеты недоступными во всех неймспейсах проекта. Вторая разрешает `postgres` в `my-project-db`: пакет, который разрешает одна политика, доступен, даже если другая политика задаёт `availabilityDefault: None`.

Условие `NewNamespacesCovered` в статусе второй политики имеет значение `False`, потому что новый дополнительный неймспейс проекта не соответствует её `namespaceSelector`. Такие неймспейсы охватывает первая политика, поэтому пакеты в них недоступны.

## Запрет отдельных пакетов

Чтобы сделать недоступными отдельные пакеты и оставить доступными остальные, перечислите их в [`denied`](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-denied). Следующая политика делает пакет `experimental-db` недоступным во всех проектах:

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

Пустой `projectSelector` выбирает все проекты. Запрещённый пакет недоступен, даже если другая политика его разрешает.

## Пакеты в шаблоне проекта и в проекте

Те же правила можно задать в `spec.resources` шаблона проекта, чтобы они действовали во всех проектах, созданных из этого шаблона, или в [`spec.grants.resources`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-grants) отдельного объекта Project. Например, чтобы разрешить в проектах шаблона только `postgres` и `redis`, добавьте в шаблон следующую запись:

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

Пакеты, доступные в неймспейсе, перечислены в объекте [AvailableClusterResource](/modules/multitenancy-manager/cr.html#availableclusterresource) `applicationpackages` этого неймспейса. Этот же список видят пользователи проекта:

```shell
d8 k get available applicationpackages -n <NAMESPACE> -o yaml
```

Запросы компонентов платформы и участников группы `system:masters` не проверяются. Чтобы проверить политику, отправьте запрос от имени пользователя проекта, например серверный пробный запуск (dry run), который ничего не создаёт:

```shell
d8 k apply -f application.yaml --dry-run=server --as <USER_NAME>
```

## Приложения, установленные до ограничения

Политика не удаляет установленные приложения. При изменении Application его пакет проверяется, только если меняется сам пакет, поэтому приложение, установленное до того, как его пакет стал недоступен проекту, продолжает работать.

Такие приложения учитывает метрика `d8_cluster_objects_grant_violated`, и срабатывает алерт [ClusterResourceGrantPolicyViolation](../../../reference/alerts.html#multitenancy-manager-clusterresourcegrantpolicyviolation). Алерт перестаёт срабатывать, когда пакет снова становится доступен проекту или Application удаляют.

## Ограничения

- Политики действуют только в неймспейсах проектов. На неймспейсы виртуальных проектов `default` и `deckhouse`, например на неймспейс `default`, они не распространяются, поэтому там можно установить любой пакет.
- Политика выбирает пакеты по имени. Она не ограничивает репозиторий, из которого устанавливается доступный пакет (`spec.packageRepositoryName`), и его версию.
- Политика не скрывает пакеты: любой аутентифицированный пользователь может читать объекты ApplicationPackageVersion всех пакетов, в том числе недоступных проекту. В поле `status.usedBy` версии перечислены приложения, установленные из неё, и их неймспейсы во всех проектах.
- Пока контроллер модуля `multitenancy-manager` недоступен, объекты Application в неймспейсах проектов нельзя создать или изменить. Чтобы контроллер оставался доступным, включите [`highAvailability`](/modules/multitenancy-manager/configuration.html#parameters-highavailability).
