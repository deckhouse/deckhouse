---
title: Установка и управление приложениями
permalink: ru/user/marketplace/applications.html
description: "Установка, обновление и удаление приложений в Deckhouse Kubernetes Platform Marketplace. Просмотр доступных версий пакетов, создание Application, проверка условий статуса и управление несколькими экземплярами."
lang: ru
search: Application, install application, application conditions, установка приложения, условия приложения, обновление приложения
---

## Просмотр доступных версий пакетов

Для получения списка всех доступных версий пакетов выполните следующую команду (можно использовать сокращённое имя — `apv`):

```bash
d8 k get apv
```

Пример вывода:

<!-- markdownlint-disable MD031 -->
```console
NAME                        PACKAGE   REPOSITORY    TRANSITIONTIME   METADATALOADED   USEDBY
my-registry-redis-v7.2.0    redis     my-registry   2d               True             1
my-registry-redis-v7.3.0    redis     my-registry   5h               True
my-registry-pg-v15.0.0      postgres  my-registry   2d               True             2
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

Для фильтрации по имени пакета используйте следующую команду (в примере фильтруются версии пакета `redis`):

```bash
d8 k get apv -l package=redis
```

Для просмотра реестров, в которых доступен пакет, используйте следующую команду (можно использовать сокращённое имя — `ap`):

```bash
d8 k get ap redis \
  -o jsonpath='{.status.availableRepositories}'
```

{% alert level="info" %}
Устанавливать можно только версии с `MetadataLoaded=True`. Это означает, что OpenAPI-схема пакета, описание и требования успешно загружены из реестра. Пакет с `MetadataLoaded=False` не может быть установлен до получения метаданных.
{% endalert %}

## Установка приложения

Для установки приложения создайте объект [Application](../../reference/api/cr.html#application) в нужном неймспейсе.

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
  # Если подключён только один репозиторий с этим пакетом, можно не указывать
  packageRepositoryName: my-registry
  settings:
    replicas: 3
    maxmemory: "256mb"
```

{% alert level="info" %}
`spec.settings` проверяется по OpenAPI-схеме, определённой в пакете. Если схема отклонила настройки, Application не будет создан. Детали схемы можно посмотреть в соответствующем [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion).
{% endalert %}

### Ограничения на имена

Имя Application (`metadata.name`) должно быть **не более 24 символов**. Это обязательно, поскольку все поды получают префикс из имени экземпляра: 24 символа имени экземпляра + 24 символа имени ресурса + 15 символов суффикса Deployment укладываются в ограничение Kubernetes на имя пода в 63 символа.

## Проверка статуса приложения

Для получения краткой информации о статусе приложения выполните (можно использовать сокращённое имя — `app`):

```bash
d8 k get app -n <NAMESPACE> <APPLICATION_NAME>
```

Пример вывода:

```console
NAME          PACKAGE   VERSION   INSTALLED   READY   MESSAGE
redis-cache   redis     v7.2.0    True        True
```

Для получения полного статуса, включая условия (conditions), выполните:

```bash
d8 k get app -n <NAMESPACE> <APPLICATION_NAME> -o yaml
```

### Условия (Conditions)

Состояние приложения детально описывается через набор условий:

| Условие | Значение |
|---|---|
| `Installed` | Пакет скачан, манифесты и хуки применены при первичной установке |
| `UpdateInstalled` | Новая версия скачана, манифесты и хуки применены при обновлении |
| `ConfigurationApplied` | Пользовательские настройки успешно применены |
| `Scaled` | Все реплики подов находятся в состоянии Ready |
| `Managed` | Приложение корректно управляется DKP |
| `Ready` | Приложение полностью готово к работе |

Для быстрого просмотра всех условий используйте следующую команду:

```bash
d8 k get app -n <NAMESPACE> <APPLICATION_NAME> \
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

### Summary

Поле `status.summary` содержит краткое описание текущего состояния приложения — его удобно смотреть в первую очередь при диагностике:

```yaml
status:
  summary:
    state: Updating
    message: "Update is waiting for dependent modules to converge; previous version is still serving"
    tip: "Waiting until DKP processes all dependent modules to start the update."
```

- **`state`** — текущее общее состояние приложения.
- **`message`** — объясняет, почему приложение находится в этом состоянии.
- **`tip`** — что нужно сделать для решения проблемы или чего ожидает DKP.

## Несколько экземпляров

Один и тот же пакет можно установить несколько раз в одном или разных неймспейсах — каждый с отдельным именем и настройками. Например, можно создать два экземпляра Redis — один для кеширования, другой для сессий:

```yaml
# Экземпляр для кеширования
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
# Экземпляр для сессий
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

Все объекты Kubernetes, создаваемые приложением, получают префикс из имени экземпляра — например, `redis-cache-deployment` и `redis-sessions-deployment` — что исключает конфликты имён.

## Обновление приложения

Обновление выполняется вручную: измените значение `spec.packageVersion` на нужную версию и примените изменение:

```bash
d8 k patch app -n <NAMESPACE> <APPLICATION_NAME> --type=merge -p '{"spec":{"packageVersion":"v7.3.0"}}'
```

Пока обновление выполняется, условие `UpdateInstalled` будет иметь значение `False` с `reason: Pending`. После успешного завершения оно станет `True`. До завершения обновления продолжает работать предыдущая версия.

Если указанная версия не существует в репозитории, `UpdateInstalled` становится `False` с `reason: UpdateFailed`, а текущая версия продолжает работу.

{% alert level="warning" %}
Указание более ранней версии приложения (downgrade) допускается, но DKP не применяет никакую логику миграции при откате. Убедитесь в совместимость настроек с целевой версией перед применением изменения, при необходимости.
{% endalert %}

## Удаление приложения

Для удаления приложения удалите объект Application. Например:

```bash
d8 k delete app -n <NAMESPACE> <APPLICATION_NAME>
```

При удалении Application, все созданные им объекты Kubernetes (если они не защищены аннотациями `helm.sh/resource-policy: keep` или `werf.io/ownership: anyone` в шаблонах пакета), будут удалены.

## FAQ

### Можно ли обновлять приложение автоматически?

Нет. В текущей реализации обновления требуют ручного изменения `spec.packageVersion`. Автоматические обновления через release channels запланированы на будущие версии.

### Может ли Application зависеть от другого Application?  

Нет. Application может объявлять зависимости только от модулей (через `requirements.modules` в `package.yaml`). Это архитектурное ограничение, обеспечивающее изоляцию экземпляров.

### Можно ли установить одно приложение в разных неймспейсах?  

Да. Создайте объекты Application с одинаковыми `packageName` и `packageVersion` в разных неймспейсах — каждый будет полностью независимым экземпляром.
