---
title: Репозитории пакетов
permalink: ru/admin/configuration/marketplace/package-repository.html
description: "Подключение реестра пакетов к Deckhouse Kubernetes Platform Marketplace через PackageRepository. Настройка аутентификации, интервала сканирования и мониторинг статуса репозитория."
lang: ru
search: PackageRepository, package repository, registry packages, репозиторий пакетов, реестр пакетов, сканирование
---

Подключение Deckhouse Kubernetes Platform (DKP) к container registry, содержащему пакеты приложений, выполняется с помощью [PackageRepository](../../../reference/api/cr.html#packagerepository). После подключения DKP автоматически сканирует реестр и создаёт объекты [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) для каждой обнаруженной версии пакета.

Пример манифеста PackageRepository:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: PackageRepository
metadata:
  name: my-registry
spec:
  registry:
    repo: registry.example.com/packages
    scheme: HTTPS
    dockerCfg: <base64-encoded-docker-config>
```

## Управление аутентификацией и интервалом сканирования

### Аутентификация

Для аутентификации в реестре может использоваться один из следующих способов:

- **`dockerCfg`**: Docker-конфиг в формате base64 (формат `~/.docker/config.json`). Предпочтителен, если реестр использует токен-based аутентификацию.
- **`login` + `password`**: явные учётные данные.

  ```yaml
  spec:
    registry:
      repo: registry.example.com/packages
      scheme: HTTPS
      login: my-user
      password: my-password
  ```

Если реестр использует самоподписанный TLS-сертификат, передайте его через параметр `ca`:

```yaml
spec:
  registry:
    repo: registry.example.com/packages
    scheme: HTTPS
    dockerCfg: <base64-encoded-docker-config>
    ca: |
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
```

### Интервал сканирования

По умолчанию DKP пересканирует реестр каждые **6 часов**. Переопределить интервал можно с помощью параметра `scanInterval`:

```yaml
spec:
  registry:
    repo: registry.example.com/packages
  scanInterval: 1h30m
```

## Проверка состояния репозитория

Состояние репозитория отображается в статусе объекта PackageRepository.

Для вывода краткой информации о статусе, используйте следующую команду:

```bash
d8 k get packagerepository <REPOSITORY_NAME>
```

Колонки вывода:

| Колонка | Описание |
|---|---|
| `Phase` | Текущее состояние репозитория |
| `Scan` | Время последнего сканирования |
| `MSG` | Сообщение из условия последнего сканирования |
| `Packages` | Общее количество обнаруженных пакетов (скрыто по умолчанию, используйте `-o wide`) |

Для получения детальной информации о статусе, используйте следующую команду:

```bash
d8 k get packagerepository <REPOSITORY_NAME> -o yaml
```

Ключевые поля статуса:

| Поле | Описание |
|---|---|
| `status.phase` | Текущая фаза репозитория |
| `status.lastScanTime` | Время последнего сканирования с любым результатом |
| `status.lastChangeTime` | Время последнего сканирования, которое нашло хотя бы одну новую версию |
| `status.lastNewVersions` | Количество новых версий, найденных при последнем сканировании |
| `status.packagesCount` | Общее число пакетов в репозитории |
| `status.packages[]` | Список пакетов с полями `name` и `type` |
| `status.conditions` | Детальные условия, включая `LastScanSucceeded` |

Условие `LastScanSucceeded`:

```bash
d8 k get packagerepository my-registry \
  -o jsonpath='{.status.conditions[?(@.type=="LastScanSucceeded")].message}'
```

## Просмотр обнаруженных версий пакетов

После успешного сканирования в кластере появляются объекты [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) (можно использовать сокращенное имя — `apv`):

```bash
d8 k get apv
```

Пример вывода:

<!-- markdownlint-disable MD031 -->
```console
NAME                           PACKAGE     REPOSITORY   TRANSITIONTIME   METADATALOADED   MESSAGE   USEDBY
my-registry-redis-v7.2.0       redis       my-registry  5m               True
my-registry-postgres-v15.0.0   postgres    my-registry  5m               True
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

Фильтрация по имени пакета, можно использовать следующую команду (в примере фильтруются версии пакета `redis`):

```bash
d8 k get apv -l package=redis
```

{% alert level="info" %}
`MetadataLoaded=True` означает, что OpenAPI-схема пакета, описание и требования успешно загружены из реестра. Пакет с `MetadataLoaded=False` не может быть установлен до получения метаданных.
{% endalert %}
