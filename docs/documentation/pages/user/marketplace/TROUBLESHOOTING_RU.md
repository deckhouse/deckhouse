---
title: Диагностика
permalink: ru/user/marketplace/troubleshooting.html
description: "Диагностика и устранение проблем с приложениями Deckhouse Platform Marketplace. Проверка наличия CRD, чтение условий и summary приложения, просмотр логов."
lang: ru
search: Application troubleshooting, application conditions, диагностика приложений, условия приложения, логи приложения
---

В этом разделе описан порядок решения типичных проблем при работе с приложениями [Marketplace](../../glossary.html#marketplace).

## Проверка наличия кастомных ресурсов Marketplace

Если команда `d8 k get applications` возвращает ошибку о том, что такого типа ресурсов нет, возможно, кастомные ресурсы Marketplace не установлены в кластере. Чтобы это проверить, выполните следующую команду:

```bash
d8 k get crd | grep -E 'application|package'
```

Ожидаемый вывод:

<!-- markdownlint-disable MD031 -->
```console
applicationpackages.deckhouse.io                     2026-02-10T14:54:41Z
applicationpackageversions.deckhouse.io              2026-02-10T14:54:41Z
applications.deckhouse.io                            2026-02-10T14:54:41Z
modulepackages.deckhouse.io                          2026-02-10T14:54:41Z
modulepackageversions.deckhouse.io                   2026-02-10T14:54:41Z
packagerepositories.deckhouse.io                     2026-02-10T14:54:41Z
packagerepositoryoperations.deckhouse.io             2026-02-10T14:54:41Z
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

Если каких-то ресурсов нет, обратитесь к администратору кластера. Для Marketplace нужна Deckhouse Platform (DP) версии 1.76 или выше.

## Быстрая диагностика приложения

Быстро понять причину сбоев в приложении можно по содержанию полей в секции `status.summary` ресурса Application. Для быстрой диагностики выполните следующую команду:

```bash
d8 k get applications -n <NAMESPACE> <APPLICATION_NAME> -o yaml | grep -A5 'summary:'
```

Пример вывода:

```yaml
summary:
  state: Updating
  message: "Update is waiting for dependent modules to converge; previous version is still serving"
  tip: "Wait — the previous version is still working. The update will continue automatically once dependent modules converge."
```

Описание полей `status.summary` приведено [в подразделе про ресурс Application](../../reference/api/cr.html#application-v1alpha1-status-summary).

## Чтение отдельных условий

Чтобы посмотреть состояние приложения подробнее, выполните следующую команду:

```bash
d8 k get applications -n <NAMESPACE> <APPLICATION_NAME> \
  -o jsonpath='{range .status.conditions[*]}{.type}: {.status} ({.reason}) - {.message}{"\n"}{end}'
```

Пример вывода для обновления, которое ожидает выполнения:

```console
Installed: True (Installed) -
UpdateInstalled: False (Pending) - waiting for processing
ConfigurationApplied: True (ConfigurationApplied) -
Managed: True (Managed) -
Scaled: True (Scaled) -
Ready: True (Ready) -
```

В этом примере `Installed: True` означает, что работает ранее установленная версия приложения, а `UpdateInstalled: False` с причиной `Pending` — что обновление ожидает выполнения, например, пока не будут готовы модули, от которых зависит приложение. Установленная версия указана [в поле `status.currentVersion.version`](../../reference/api/cr.html#application-v1alpha1-status-currentversion-version).

## Просмотр логов DP

Если условий недостаточно для диагностики, попросите администратора кластера проверить лог DP. Лог доступен только в неймспейсе `d8-system`:

```bash
d8 k -n d8-system logs svc/deckhouse-leader -c deckhouse | grep '<NAMESPACE>.<APPLICATION_NAME>'
```

## Просмотр логов подов приложения

Объекты приложения называются в формате `d8a-<APPLICATION_NAME>-<SUFFIX>`. Чтобы получить список Deployment и StatefulSet приложения, выполните следующую команду:

```bash
d8 k get deployments,statefulsets -n <NAMESPACE> -l packages.deckhouse.io/instance=<APPLICATION_NAME>
```

Имена подов этих рабочих нагрузок начинаются с того же префикса. Чтобы получить их список, выполните:

```bash
d8 k get pods -n <NAMESPACE> | grep 'd8a-<APPLICATION_NAME>-'
```

Чтобы посмотреть логи конкретного пода, выполните:

```bash
d8 k logs -n <NAMESPACE> <POD_NAME>
```

Чтобы посмотреть логи Deployment приложения, выполните:

```bash
d8 k logs -n <NAMESPACE> deployments/d8a-<APPLICATION_NAME>-<SUFFIX>
```

## Частые причины условий

В таблице перечислены наиболее частые причины отображения перечисленных [условий](../../reference/api/cr.html#application-v1alpha1-status-conditions-reason).

| Причина | Условия | Значение и что проверить |
|---|---|---|
| `Pending` | `Installed`, `UpdateInstalled` | DP ждёт возможности установить версию, например, пока не будут включены модули из `requirements.modules` [пакета](../../glossary.html#marketplace-package). Попросите администратора проверить эти модули |
| `RequirementsUnmet` | `Installed` | Кластер не удовлетворяет требованиям пакета. В сообщении указано невыполненное требование |
| `DownloadFailed` | `Installed`, `UpdateInstalled`, `ConfigurationApplied`, `Managed`, `Ready` | DP не удаётся загрузить версию пакета. Попросите администратора проверить PackageRepository и доступ к репозиторию |
| `LoadFromFilesystemFailed` | `Installed`, `UpdateInstalled`, `Ready` | DP не удаётся прочитать загруженный пакет. Обратитесь к разработчику пакета |
| `SettingsInvalid` | `Installed`, `UpdateInstalled`, `ConfigurationApplied` | Настройки не прошли проверку. Сверьте `spec.settings` со схемой настроек в [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) |
| `HookInitializationFailed`, `HookFailed` | `Installed`, `UpdateInstalled`, `ConfigurationApplied`, `Managed`, `Ready` | Хук пакета завершился с ошибкой. Текст ошибки указан в сообщении |
| `ManifestsApplyFailed` | `Installed`, `UpdateInstalled`, `ConfigurationApplied`, `Managed`, `Ready` | DP не удаётся применить манифесты пакета. Проверьте сообщение и события в неймспейсе |
| `ApplyingManifests`, `SettingsChanged` | `UpdateInstalled`, `ConfigurationApplied`, `Managed`, `Ready` | Не является ошибкой. DP применяет манифесты или изменённые настройки |
| `Reconciling` | `Scaled` | Идёт развёртывание рабочей нагрузки. Имя нагрузки указано в сообщении |
| `Degraded` | `Scaled` | Рабочую нагрузку не удалось развернуть. Проверьте события и поды рабочей нагрузки с помощью команды `d8 k describe` |
| `NoResourceReconciliation` | `Managed` | Приложение находится в [режиме обслуживания](../../glossary.html#no-resource-reconciliation) (поле `spec.maintenance`) |
| `Deleting` | Все | Приложение удаляется |
