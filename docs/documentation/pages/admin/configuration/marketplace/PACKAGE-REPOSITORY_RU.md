---
title: Репозитории пакетов
permalink: ru/admin/configuration/marketplace/package-repository.html
description: "Подключение хранилища образов с пакетами к Deckhouse Platform Marketplace с помощью PackageRepository. Настройка аутентификации, интервала сканирования и мониторинг статуса репозитория."
lang: ru
search: PackageRepository, package repository, container registry, репозиторий пакетов, хранилище образов, сканирование
---

Чтобы подключить Deckhouse Platform (DP) к хранилищу образов с пакетами приложений, используйте ресурс [PackageRepository](../../../reference/api/cr.html#packagerepository). После создания ресурса DP сканирует хранилище образов и создаёт объект [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) для каждой обнаруженной версии пакета.

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
    dockerCfg: <BASE64_ENCODED_DOCKER_CONFIG>
```

## Управление аутентификацией и интервалом сканирования

### Аутентификация

Для аутентификации в хранилище образов используйте один из следующих способов:

- **`dockerCfg`**: конфигурация Docker в формате `~/.docker/config.json` в кодировке Base64. Она должна содержать запись `auths` для хоста хранилища образов: DP использует имя пользователя и пароль из этой записи.
- **`login` + `password`**: явно заданные учётные данные. Если указаны оба способа, используются `login` и `password`.

  ```yaml
  spec:
    registry:
      repo: registry.example.com/packages
      scheme: HTTPS
      login: my-user
      password: my-password
  ```

Если хранилище образов использует самоподписанный TLS-сертификат, укажите CA-сертификат в параметре `ca`:

```yaml
spec:
  registry:
    repo: registry.example.com/packages
    scheme: HTTPS
    dockerCfg: <BASE64_ENCODED_DOCKER_CONFIG>
    ca: |
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
```

### Интервал сканирования

По умолчанию DP сканирует хранилище образов каждые **6 часов**. Чтобы изменить интервал, задайте параметр `scanInterval`. Минимальный интервал — 3 минуты:

```yaml
spec:
  registry:
    repo: registry.example.com/packages
  scanInterval: 1h30m
```

Когда запускается сканирование и как запустить его вручную, описано в разделе [«Сканирование»](scanning.html#запуск-сканирования-вручную).

## Проверка состояния репозитория

Состояние репозитория отображается в статусе объекта PackageRepository.

Чтобы вывести краткую информацию о статусе, выполните следующую команду:

```bash
d8 k get packagerepository <REPOSITORY_NAME>
```

Колонки вывода:

| Колонка | Описание |
|---|---|
| `Phase` | Фаза репозитория: `Active` после первого успешного сканирования |
| `Scan` | Время последнего успешного сканирования |
| `Repository` | Адрес репозитория (`spec.registry.repo`) |
| `Packages` | Количество пакетов в репозитории |
| `MSG` | Сообщение условия `LastScanSucceeded`, например, ошибка последнего сканирования |

Чтобы получить подробную информацию о статусе, выполните следующую команду:

```bash
d8 k get packagerepository <REPOSITORY_NAME> -o yaml
```

Ключевые поля статуса:

| Поле | Описание |
|---|---|
| `status.phase` | Фаза репозитория: `Active` после первого успешного сканирования |
| `status.lastScanTime` | Время последнего успешного сканирования. Неудачное сканирование его не обновляет |
| `status.lastChangeTime` | Время последнего сканирования, которое нашло хотя бы одну новую версию |
| `status.lastNewVersions` | Количество новых версий, найденных при последнем успешном сканировании |
| `status.packagesCount` | Общее количество пакетов в репозитории |
| `status.packages[]` | Список пакетов с полями `name` и `type` |
| `status.conditions` | Подробные условия, включая `LastScanSucceeded` |

Чтобы проверить результат последнего сканирования, посмотрите условие `LastScanSucceeded`:

```bash
d8 k get packagerepository <REPOSITORY_NAME> \
  -o jsonpath='{.status.conditions[?(@.type=="LastScanSucceeded")]}'
```

Условие имеет статус `True`, если последнее сканирование прошло успешно. Иначе оно имеет статус `False`, а его сообщение содержит ошибку.

## Просмотр обнаруженных версий пакетов

После успешного сканирования в кластере появляются объекты [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) (можно использовать сокращённое имя — `apv`):

```bash
d8 k get apv
```

Пример вывода:

<!-- markdownlint-disable MD031 -->
```console
NAME                           PACKAGE    REPOSITORY    METADATALOADED   USEDBY   AGE
my-registry-redis-v7.2.0       redis      my-registry   True                      5m
my-registry-postgres-v15.0.0   postgres   my-registry   True                      5m
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

Чтобы отфильтровать версии по имени пакета, выполните следующую команду (в примере — версии пакета `redis`):

```bash
d8 k get apv -l packages.deckhouse.io/package=redis
```

{% alert level="info" %}
`MetadataLoaded=True` означает, что OpenAPI-схема пакета, описание и требования успешно загружены из хранилища образов. Версию пакета с `MetadataLoaded=False` нельзя установить, пока не загружены метаданные.
{% endalert %}
