---
title: Сканирование
permalink: ru/admin/configuration/marketplace/scanning.html
description: "Мониторинг и управление операциями сканирования репозиториев пакетов в Deckhouse Kubernetes Platform Marketplace. Просмотр истории сканирований, проверка прогресса и запуск сканирования вручную через PackageRepositoryOperation."
lang: ru
search: PackageRepositoryOperation, scanning, scan operation, сканирование, операция сканирования, репозиторий пакетов
---

Deckhouse Kubernetes Platform (DKP) использует [PackageRepositoryOperation](../../../reference/api/cr.html#packagerepositoryoperation) для сканирования реестров пакетов. Каждая операция сканирования обнаруживает новые версии пакетов и создаёт или обновляет объекты [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion). Операции создаются автоматически по расписанию или могут создаваться вручную при необходимости.

## Просмотр операций сканирования

Для просмотра всех операций сканирования используйте команду (можно использовать `pro` — сокращенное имя для `packagerepositoryoperations`):

```bash
d8 k get pro
```

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
| `Completed` | Завершена ли операция (`True` / `False`) |
| `MSG` | Сообщение из условия `Completed` |
| `CompletionTime` | Время завершения операции |

Для фильтрации операций по конкретному репозиторию используйте следующую команду:

```bash
d8 k get pro -l packages.deckhouse.io/repository=<REPO_NAME>
```

## Детали операции сканирования

Для получения полных результатов сканирования с детализацией по пакетам используйте следующую команду:

```bash
d8 k get pro <имя-операции> -o yaml
```

Ключевые поля статуса:

| Поле | Описание |
|---|---|
| `status.startTime` | Время начала операции |
| `status.completionTime` | Время завершения операции |
| `status.packages.total` | Всего найдено пакетов |
| `status.packages.processedOverall` | Всего успешно обработано пакетов |
| `status.packages.newVersionsOverall` | Суммарное количество новых версий по всем пакетам |
| `status.packages.processed[]` | Результаты по каждому пакету: `name`, `type`, `foundVersions`, `newVersions` |
| `status.packages.failed[]` | Пакеты с ошибками: `name`, `errors[]` с полями `version` и `message` |
| `status.packages.discovered[]` | Пакеты, впервые обнаруженные в этой операции |

Пример команды для просмотра пакетов с ошибками:

```bash
d8 k get pro <имя-операции> \
  -o jsonpath='{range .status.packages.failed[*]}{.name}: {range .errors[*]}{.version} - {.message}{"\n"}{end}{end}'
```

## Запуск сканирования вручную

По умолчанию каждый [PackageRepository](../../../reference/api/cr.html#packagerepository) создаёт новую операцию сканирования через **6 часов** после завершения предыдущей (настраивается через `spec.scanInterval`).

Для немедленного сканирования создайте PackageRepositoryOperation вручную, по следующему примеру:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: PackageRepositoryOperation
spec:
  packageRepositoryName: my-registry
  type: Update
  update:
    fullScan: true
    timeout: 5m
```

{% alert level="info" %}
Используйте `generateName` вместо `name`, чтобы создавать несколько операций без конфликтов имён.
{% endalert %}

Альтернативно, для создания операции сканирования, можно использовать следующую команду:

```bash
d8 system package scan <REPO_NAME>
```

Эта команда создаёт PackageRepositoryOperation с `spec.type: Update` и `spec.update.fullScan: true`.

### Флаг `fullScan`

| Значение | Поведение |
|---|---|
| `true` | Повторно проверяет все теги в реестре, включая уже известные |
| `false` (по умолчанию) | Обрабатывает только теги, добавленные с момента последнего сканирования (инкрементальное) |

Используйте `fullScan: true`, если вы подозреваете, что реестр был изменён вне обычного рабочего процесса, или когда в объектах [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) отсутствуют версии, которые есть в реестре.
