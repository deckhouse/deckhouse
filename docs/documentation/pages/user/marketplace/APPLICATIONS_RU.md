---
title: Установка и управление приложениями
permalink: ru/user/marketplace/applications.html
description: "Установка, обновление и удаление приложений в Deckhouse Platform Marketplace. Просмотр доступных версий пакетов, создание Application, проверка условий статуса и управление несколькими экземплярами."
lang: ru
search: Application, install application, application conditions, установка приложения, условия приложения, обновление приложения, удаление приложения
---

В этом разделе описан порядок просмотра доступных версий пакетов а также, установка, обновление и удаление приложений.

## Просмотр доступных версий пакетов

Чтобы получить список всех доступных версий пакетов, выполните следующую команду (можно использовать сокращённое имя `apv` для объектов ApplicationPackageVersion):

```bash
d8 k get apv
```

Пример вывода:

<!-- markdownlint-disable MD031 -->
```console
NAME                           PACKAGE    REPOSITORY    METADATALOADED   USEDBY   AGE
my-registry-redis-v7.2.0       redis      my-registry   True             1        2d
my-registry-redis-v7.3.0       redis      my-registry   True                      5h
my-registry-postgres-v15.0.0   postgres   my-registry   True             2        2d
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

В имени объекта ApplicationPackageVersion используется следующий формат: `<REPOSITORY_NAME>-<PACKAGE_NAME>-<PACKAGE_VERSION>`. Чтобы увидеть также время загрузки метаданных и ошибку загрузки при её наличии, добавьте в команду `-o wide`.

Чтобы отфильтровать версии по имени пакета, выполните следующую команду (в примере — версии пакета `redis`):

```bash
d8 k get apv -l packages.deckhouse.io/package=redis
```

Чтобы узнать, в каких репозиториях доступен пакет, выполните следующую команду (можно использовать сокращённое имя — `ap` для объектов ApplicationPackage):

```bash
d8 k get ap redis \
  -o jsonpath='{.status.availableRepositories}'
```

{% alert level="info" %}
`MetadataLoaded=True` в выводе означает, что OpenAPI-схема пакета, описание и требования успешно загружены из репозитория. Версию пакета с `MetadataLoaded=False` нельзя установить, пока не загружены метаданные.
{% endalert %}

## Установка приложения

Чтобы установить приложение, создайте объект [Application](../../reference/api/cr.html#application) в нужном неймспейсе.

Пример манифеста для установки Redis из пакета `redis` версии `v7.2.0` с настройкой `maxmemory`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Application
metadata:
  name: redis-cache
  namespace: my-app
spec:
  packageName: redis
  packageVersion: "v7.2.0"
  # Имя PackageRepository, в котором находится пакет.
  packageRepositoryName: my-registry
  settings:
    replicas: 3
    maxmemory: "256mb"
```

{% alert level="info" %}
Секция настроек `spec.settings` проверяется по OpenAPI-схеме, определённой в пакете. Если схема отклоняет настройки, Application не создаётся. Схема опубликована [в поле `status.packageSchemas.settingsSchema`](../../reference/api/cr.html#applicationpackageversion-v1alpha1-status-packageschemas-settingsschema) объекта ApplicationPackageVersion пакета.
{% endalert %}

### Ограничения на имена

Имя Application (`metadata.name`) должно быть **не длиннее 24 символов**. Application с более длинным именем отклоняется при создании. Имя экземпляра входит в имена всех объектов приложения. Имена объектов создаются в формате `d8a-<INSTANCE_NAME>-<SUFFIX>`, например, `d8a-redis-cache-server`, и должны укладываться в ограничения Kubernetes на длину имён (подробнее — [в разделе «Архитектура»](../../architecture/marketplace/concepts.html#ограничения-на-имена)).

## Проверка статуса приложения

Чтобы получить краткую информацию о статусе приложения, выполните следующую команду:

```bash
d8 k get applications -n <NAMESPACE> <APPLICATION_NAME>
```

Пример вывода:

```console
NAME          PACKAGE   VERSION   STATE   MESSAGE   AGE
redis-cache   redis     v7.2.0    Ready             5m
```

Чтобы увидеть также репозиторий и условия `Installed` и `Ready`, добавьте `-o wide`.

Чтобы получить полный статус, включая условия (conditions), выполните следующую команду:

```bash
d8 k get applications -n <NAMESPACE> <APPLICATION_NAME> -o yaml
```

### Условия

Состояние приложения подробно описывается набором условий:

| Условие | Значение |
|---|---|
| `Installed` | Первичная установка завершена: пакет загружен, хуки выполнены, манифесты применены, а Deployment и StatefulSet приложения готовы |
| `UpdateInstalled` | Новая версия установлена: она загружена, хуки выполнены, манифесты применены. Условие появляется после первого изменения версии пакета |
| `ConfigurationApplied` | Текущая конфигурация применена: настройки, хуки и манифесты |
| `Scaled` | Все Deployment и StatefulSet приложения развёрнуты и имеют нужное количество готовых реплик |
| `Managed` | DP управляет приложением. `False` в [режиме обслуживания](../../architecture/marketplace/lifecycle.html#режим-обслуживания) или если DP не может поддерживать приложение в управляемом состоянии, например, из-за ошибок хуков или манифестов |
| `Ready` | Приложение готово к работе. Во время обновления условие может оставаться `True`, пока работает предыдущая версия |

Пока первичная установка не завершена, у приложения есть только условие `Installed`. Остальные условия появляются, когда оно становится `True`, и снова убираются, пока оно `False`, например, если отключён модуль, от которого зависит приложение. Пока приложение удаляется, все условия имеют статус `False` с причиной `Deleting`.

Чтобы быстро просмотреть все условия, выполните следующую команду:

```bash
d8 k get applications -n <NAMESPACE> <APPLICATION_NAME> \
  -o jsonpath='{range .status.conditions[*]}{.type}: {.status} ({.reason}){"\n"}{end}'
```

Пример вывода:

```console
Installed: True (Installed)
UpdateInstalled: False (Pending)
ConfigurationApplied: True (ConfigurationApplied)
Managed: True (Managed)
Scaled: True (Scaled)
Ready: True (Ready)
```

### Краткое описание состояния

[Секция `status.summary`](../../reference/api/cr.html#application-v1alpha1-status-summary) содержит краткое описание текущего состояния приложения. При диагностике используйте эту информацию в первую очередь:

```yaml
status:
  summary:
    state: Updating
    message: "Update is waiting for dependent modules to converge; previous version is still serving"
    tip: "Wait — the previous version is still working. The update will continue automatically once dependent modules converge."
```

Описание полей `status.summary` приведено [в подразделе про ресурс Application](../../reference/api/cr.html#application-v1alpha1-status-summary).

## Установка нескольких экземпляров

Один и тот же пакет можно установить несколько раз в одном или разных неймспейсах, каждый раз с отдельным именем и настройками. Например, можно создать два экземпляра Redis: один для кеширования, другой для сессий:

```yaml
# Экземпляр для кеширования.
apiVersion: deckhouse.io/v1alpha1
kind: Application
metadata:
  name: redis-cache
  namespace: team-alpha
spec:
  packageName: redis
  packageRepositoryName: my-registry
  packageVersion: "v7.2.0"
  settings:
    maxmemory: "512mb"
---
# Экземпляр для хранения сессий.
apiVersion: deckhouse.io/v1alpha1
kind: Application
metadata:
  name: redis-sessions
  namespace: team-alpha
spec:
  packageName: redis
  packageRepositoryName: my-registry
  packageVersion: "v7.2.0"
  settings:
    maxmemory: "128mb"
```

Объекты каждого экземпляра называются в формате `d8a-<INSTANCE_NAME>-<SUFFIX>`. Например, `d8a-redis-cache-server` и `d8a-redis-sessions-server`. Благодаря этому, имена не конфликтуют.

## Обновление приложения

Приложения обновляются вручную. Чтобы обновить приложение, измените значение [поля `spec.packageVersion`](../../reference/api/cr.html#application-v1alpha1-spec-packageversion) на нужную версию и примените изменение:

```bash
d8 k patch applications -n <NAMESPACE> <APPLICATION_NAME> --type=merge -p '{"spec":{"packageVersion":"v7.3.0"}}'
```

Пока идёт обновление, в условии `UpdateInstalled` указан статус `False` с причиной `Pending`, а затем с причиной `ApplyingManifests`, пока применяются манифесты новой версии. После успешного обновления условие становится `True`. Предыдущая версия работает, пока обновление не завершится.

Если указанной версии нет в репозитории, изменение отклоняется, а текущая версия продолжает работать. Если Deckhouse Platform (DP) не удаётся загрузить новую версию, `UpdateInstalled` становится `False` с причиной `DownloadFailed`, а текущая версия продолжает работать.

{% alert level="warning" %}
Можно указать более раннюю версию, чтобы выполнить downgrade, но при откате DP не применяет никакой логики миграции. При необходимости перед применением изменения убедитесь, что настройки совместимы с целевой версией.
{% endalert %}

## Удаление приложения

Чтобы удалить приложение, удалите объект Application. Например:

```bash
d8 k delete applications -n <NAMESPACE> <APPLICATION_NAME>
```

При удалении Application DP удаляет объекты Kubernetes приложения, кроме объектов, которые шаблоны пакета защищают аннотацией `helm.sh/resource-policy: keep`. Объекты, которые приложение создаёт во время работы, и объекты, которые остаются после удаления Helm-релиза (например, Helm-хуки), удаляются, если пакет объявляет их [orphan-ресурсами](../../architecture/marketplace/lifecycle.html#orphan-ресурсы). Объект Application остаётся в кластере, пока DP не завершит удаление.

## FAQ

### Можно ли обновлять приложение автоматически?

Нет. В текущей реализации для обновления нужно вручную изменить [поле `spec.packageVersion`](../../reference/api/cr.html#application-v1alpha1-spec-packageversion). Автоматическое обновление приложений через каналы обновлений будет реализовано в одной из следующих версий.

### Может ли Application зависеть от другого Application?

Нет. Application может объявлять зависимости только от модулей (через `requirements.modules` в `package.yaml`). Это архитектурное ограничение, которое обеспечивает изоляцию экземпляров.

### Можно ли установить одно и то же приложение в разные неймспейсы?

Да. Создайте объекты Application с одинаковыми `packageName` и `packageVersion` в разных неймспейсах. Каждый из них будет полностью независимым экземпляром.
