---
title: Сканирование
permalink: ru/admin/configuration/marketplace/scanning.html
description: "Мониторинг и управление операциями сканирования репозиториев пакетов в Deckhouse Platform Marketplace. Просмотр истории сканирований, проверка прогресса и запуск сканирования вручную через PackageRepositoryOperation."
lang: ru
search: PackageRepositoryOperation, scanning, scan operation, сканирование, операция сканирования, репозиторий пакетов
---

В Deckhouse Platform (DP) сканирование репозиториев пакетов настраивается с помощью объектов [PackageRepositoryOperation](../../../reference/api/cr.html#packagerepositoryoperation). Каждая операция сканирования обнаруживает новые версии пакетов и создаёт для них объекты [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion). DP создаёт операции автоматически, но их также можно создавать вручную.

## Просмотр операций сканирования

Чтобы просмотреть список операций сканирования, выполните следующую команду (сокращённое имя PackageRepositoryOperation — `pro`):

```bash
d8 k get pro
```

DP хранит 10 последних операций для каждого репозитория и при превышении этого количества удаляет операции, начиная с наиболее старых.

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
| `Completed` | Завершена ли операция (`True` или `False`). Операция, завершившаяся с ошибкой, тоже считается завершённой. Результат определяется причиной условия `Completed`: `ScanSucceeded` или `ScanFailed` |
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

Описание полей статуса приведено [в подразделе про ресурс PackageRepositoryOperation](../../../reference/api/cr.html#packagerepositoryoperation-v1alpha1-status).

Пример команды для просмотра пакетов с ошибками:

```bash
d8 k get pro <OPERATION_NAME> \
  -o jsonpath='{range .status.packages.failed[*]}{.name}: {range .errors[*]}{.version} - {.message}{"\n"}{end}{end}'
```

## Запуск сканирования вручную

DP запускает сканирование репозитория пакетов при создании [ресурса PackageRepository](../../../reference/api/cr.html#packagerepository), изменении его спецификации, перезапуске DP, а затем с интервалом [`spec.scanInterval`](../../../reference/api/cr.html#packagerepository-v1alpha1-spec-scaninterval) (по умолчанию — 6 часов, не меньше 3 минут). Сканирование пропускается, если предыдущая операция репозитория ещё не завершилась.

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
Поле `generateName` задаёт каждой операции уникальное имя. Оно работает только с командой `d8 k create`, но не с командой `d8 k apply`.
{% endalert %}

Операцию сканирования также можно запустить следующей командой:

```bash
d8 system package scan <REPOSITORY_NAME>
```

Эта команда создаёт PackageRepositoryOperation с настройками [`spec.type: Update`](../../../reference/api/cr.html#packagerepositoryoperation-v1alpha1-spec-type) и [`spec.update.fullScan: true`](../../../reference/api/cr.html#packagerepositoryoperation-v1alpha1-spec-update-fullscan).

### Полное сканирование

Чтобы выполнить полное сканирование репозитория пакетов, укажите [параметр `spec.update.fullScan: true`](../../../reference/api/cr.html#packagerepositoryoperation-v1alpha1-spec-update-fullscan) в PackageRepositoryOperation. В этом режиме проверяются все доступные версии пакетов, а не только версии выше уже обнаруженных.

При `spec.update.fullScan: false` (по умолчанию) обрабатываются только SemVer-теги с версией выше последней версии пакета, уже обработанной в кластере.

До первого успешного сканирования автоматические операции выполняют полное сканирование репозитория. После этого при автоматическом сканировании обрабатываются только версии выше последней уже обработанной версии пакета.

Запускайте полное сканирование в следующих случаях:

- версия пакета была опубликована после более высокой версии, например, патч для предыдущей минорной версии;
- в кластере отсутствуют ресурсы [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) для версий, доступных в репозитории пакетов.
