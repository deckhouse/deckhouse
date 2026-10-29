---
title: Репозитории пакетов
permalink: ru/admin/configuration/marketplace/package-repository.html
description: "Подключение репозитория пакетов к Deckhouse Platform Marketplace с помощью PackageRepository. Настройка аутентификации, интервала сканирования и мониторинг статуса репозитория."
lang: ru
search: PackageRepository, package repository, container registry, репозиторий пакетов, хранилище образов, сканирование
---

Чтобы подключить Deckhouse Platform (DP) к репозиторию [пакетов приложений](../../../glossary.html#marketplace-package), используйте ресурс [PackageRepository](../../../reference/api/cr.html#packagerepository). После создания ресурса DP сканирует репозиторий и создаёт объект [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) для каждой обнаруженной версии пакета.

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

Для аутентификации в репозитории используйте один из следующих способов:

- [`dockerCfg`](../../../reference/api/cr.html#packagerepository-v1alpha1-spec-registry-dockercfg) — конфигурация Docker в формате `~/.docker/config.json` в кодировке Base64. Она должна содержать запись `auths` для хоста репозитория. DP использует имя пользователя и пароль из этой записи;
- [`login`](../../../reference/api/cr.html#packagerepository-v1alpha1-spec-registry-login) и [`password`](../../../reference/api/cr.html#packagerepository-v1alpha1-spec-registry-password) — явно заданные учётные данные. Если указаны оба способа, используются `login` и `password`.

  ```yaml
  spec:
    registry:
      repo: registry.example.com/packages
      scheme: HTTPS
      login: my-user
      password: my-password
  ```

Если в репозитории используется самоподписанный TLS-сертификат, укажите CA-сертификат [в параметре `ca`](../../../reference/api/cr.html#packagerepository-v1alpha1-spec-registry-ca):

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

По умолчанию DP сканирует репозиторий **каждые 6 часов**. Чтобы изменить интервал, задайте [параметр `scanInterval`](../../../reference/api/cr.html#packagerepository-v1alpha1-spec-scaninterval). Минимальный интервал — 3 минуты. Пример настройки:

```yaml
spec:
  registry:
    repo: registry.example.com/packages
  scanInterval: 1h30m
```

Подробнее о запуске и просмотре результатов сканирования — [в разделе «Сканирование»](scanning.html).

## Проверка состояния репозитория

Состояние репозитория отображается в статусе объекта PackageRepository.

Чтобы вывести краткую информацию о статусе, выполните следующую команду:

```bash
d8 k get packagerepository <REPOSITORY_NAME>
```

Колонки вывода:

| Колонка | Описание |
|---|---|
| `Phase` | Фаза репозитория. После первого успешного сканирования будет указано `Active` |
| `Scan` | Время последнего успешного сканирования |
| `Repository` | Адрес репозитория ([`spec.registry.repo`](../../../reference/api/cr.html#packagerepository-v1alpha1-spec-registry-repo)) |
| `Packages` | Количество [пакетов](../../../glossary.html#marketplace-package) в репозитории |
| `MSG` | Сообщение условия `LastScanSucceeded`, например, ошибка последнего сканирования |

Чтобы получить подробную информацию о статусе, выполните следующую команду:

```bash
d8 k get packagerepository <REPOSITORY_NAME> -o yaml
```

Описание полей статуса приведено [в подразделе про ресурс PackageRepository](../../../reference/api/cr.html#packagerepository-v1alpha1-status).

Чтобы проверить результат последнего сканирования, проверьте условие `LastScanSucceeded`:

```bash
d8 k get packagerepository <REPOSITORY_NAME> \
  -o jsonpath='{.status.conditions[?(@.type=="LastScanSucceeded")]}'
```

Если последнее сканирование прошло успешно, у условия будет статус `True`. В обратном случае будет указан статус `False`, а детали ошибки будут указаны в сообщении статуса.

## Просмотр обнаруженных версий пакетов

После успешного сканирования в кластере автоматически создаются объекты [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) (сокращённое имя — `apv`).

Чтобы вывести список всех созданных объектов ApplicationPackageVersion, выполните следующую команду:

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
`MetadataLoaded=True` в выводе означает, что OpenAPI-схема пакета, описание и требования успешно загружены из репозитория. Версию пакета с `MetadataLoaded=False` нельзя установить, пока не загружены метаданные.
{% endalert %}
