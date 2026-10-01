---
title: Сканирование
permalink: ru/admin/configuration/marketplace/scanning.html
description: "Мониторинг и управление операциями сканирования репозиториев пакетов в Deckhouse Platform Marketplace. Просмотр истории сканирований, проверка прогресса и запуск сканирования вручную через PackageRepositoryOperation."
lang: ru
search: PackageRepositoryOperation, scanning, scan operation, сканирование, операция сканирования, репозиторий пакетов
---

Deckhouse Platform (DP) сканирует репозитории пакетов с помощью объектов [PackageRepositoryOperation](../../../reference/api/cr.html#packagerepositoryoperation). Каждая операция сканирования обнаруживает новые версии пакетов и создаёт для них объекты [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion). DP создаёт операции автоматически, также их можно создавать вручную.

## Просмотр операций сканирования

Чтобы посмотреть операции сканирования, выполните следующую команду (можно использовать `pro` — сокращённое имя для `packagerepositoryoperations`):

```bash
d8 k get pro
```

DP хранит 10 последних операций каждого репозитория и удаляет более старые.

Пример вывода:

<!-- markdownlint-disable MD031 -->
```console
NAME                   COUNT   COMPLETED   MSG   COMPLETIONTIME
test-scan-1780052895   23      True              3h38m
test-scan-1780053890   23      True              3h22m
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

Колонки вывода:

| Колонка | Описание |
|---|---|
| `Count` | Общее количество пакетов, найденных при сканировании |
| `Completed` | Завершена ли операция (`True` / `False`). Операция, завершившаяся с ошибкой, тоже считается завершённой: результат определяется причиной условия `Completed` — `ScanSucceeded` или `ScanFailed` |
| `MSG` | Сообщение условия `Completed`, например, ошибка сканирования |
| `CompletionTime` | Время завершения операции |

Чтобы отфильтровать операции по репозиторию, выполните следующую команду:

```bash
d8 k get pro -l packages.deckhouse.io/repository=<REPOSITORY_NAME>
```

## Детали операции сканирования

Чтобы получить полные результаты сканирования с детализацией по пакетам, выполните следующую команду:

```bash
d8 k get pro <OPERATION_NAME> -o yaml
```

Ключевые поля статуса:

| Поле | Описание |
|---|---|
| `status.startTime` | Время начала операции |
| `status.completionTime` | Время завершения операции |
| `status.packages.total` | Общее количество найденных пакетов |
| `status.packages.processedOverall` | Количество уже обработанных пакетов, включая пакеты с ошибками |
| `status.packages.newVersionsOverall` | Суммарное количество новых версий по всем пакетам |
| `status.packages.processed[]` | Результаты по каждому пакету: `name`, `type`, `foundVersions`, `newVersions` |
| `status.packages.failed[]` | Пакеты с ошибками: `name`, `errors[]` с полями `version` и `message` |
| `status.packages.discovered[]` | Пакеты, которые ещё ожидают обработки. После завершения операции список пуст |

Пример команды для просмотра пакетов с ошибками:

```bash
d8 k get pro <OPERATION_NAME> \
  -o jsonpath='{range .status.packages.failed[*]}{.name}: {range .errors[*]}{.version} - {.message}{"\n"}{end}{end}'
```

## Запуск сканирования вручную

DP запускает сканирование [PackageRepository](../../../reference/api/cr.html#packagerepository) при создании ресурса, при изменении его спецификации, при перезапуске DP, а затем с интервалом `spec.scanInterval` (по умолчанию — 6 часов, не меньше 3 минут). Сканирование пропускается, если предыдущая операция репозитория ещё не завершилась.

Чтобы запустить сканирование немедленно, создайте PackageRepositoryOperation вручную, например:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: PackageRepositoryOperation
metadata:
  generateName: my-registry-scan-manual-
spec:
  packageRepositoryName: my-registry
  type: Update
  update:
    fullScan: true
```

{% alert level="info" %}
Поле `generateName` задаёт каждой операции уникальное имя. Оно работает только с `d8 k create`, но не с `d8 k apply`.
{% endalert %}

Операцию сканирования также можно создать следующей командой:

```bash
d8 system package scan <REPOSITORY_NAME>
```

Эта команда создаёт PackageRepositoryOperation с `spec.type: Update` и `spec.update.fullScan: true`.

### Параметр `fullScan`

| Значение | Поведение |
|---|---|
| `true` | Получает все semver-теги каждого пакета и создаёт версии, которых нет в кластере. Уже существующие версии повторно не читаются |
| `false` (по умолчанию) | Обрабатывает только теги с версией выше последней версии пакета, уже обработанной в кластере |

Автоматические операции репозитория выполняют полное сканирование до первого успешного сканирования, а затем — инкрементальное.

Используйте `fullScan: true`, если версия была опубликована после более высокой, например, патч для более старой минорной версии, или если в объектах [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) отсутствуют версии, которые есть в хранилище образов.
