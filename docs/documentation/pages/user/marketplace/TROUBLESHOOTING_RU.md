---
title: Диагностика
permalink: ru/user/marketplace/troubleshooting.html
description: "Диагностика и устранение проблем с приложениями Deckhouse Platform Marketplace. Проверка наличия CRD, чтение условий и summary приложения, просмотр логов."
lang: ru
search: Application troubleshooting, application conditions, диагностика приложений, условия приложения, логи приложения
---

## Проверка наличия CRD Marketplace

Если `d8 k get applications` возвращает ошибку о том, что такого типа ресурсов нет, возможно, CRD Marketplace не установлены. Чтобы это проверить, выполните следующую команду:

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

Если каких-то CRD нет, обратитесь к администратору кластера. Для Marketplace нужна DP версии 1.76 или выше.

## Чтение summary приложения

Быстрее всего понять, почему приложение не работает, можно по полю `status.summary`. Для этого выполните следующую команду:

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

- **`state`** — текущее общее состояние приложения: `Pending`, `Failed`, `Updating`, `Ready`, `Degraded`, `Suspended` или `Deleting`.
- **`message`** — объясняет, почему приложение находится в этом состоянии.
- **`tip`** — что нужно сделать для решения проблемы или чего ожидает DP.

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

В этом примере `Installed=True` означает, что работает ранее установленная версия приложения, а `UpdateInstalled=False` с причиной `Pending` — что обновление ожидает выполнения, например, пока не будут готовы модули, от которых зависит приложение. Установленная версия указана в поле `status.currentVersion.version`.

## Просмотр логов DP

Если условий недостаточно для диагностики, попросите администратора кластера посмотреть лог DP. Лог доступен только в неймспейсе `d8-system`:

```bash
d8 k -n d8-system logs svc/deckhouse-leader -c deckhouse | grep '<NAMESPACE>.<APPLICATION_NAME>'
```

## Просмотр логов подов приложения

Объекты приложения называются `d8a-<APPLICATION_NAME>-<SUFFIX>`. Чтобы получить список Deployment и StatefulSet приложения, выполните следующую команду:

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

| Причина | Условия | Значение и что проверить |
|---|---|---|
| `Pending` | `Installed`, `UpdateInstalled` | DP ждёт возможности установить версию, например, пока не будут включены модули из `requirements.modules` пакета. Попросите администратора проверить эти модули |
| `RequirementsUnmet` | `Installed` | Кластер не удовлетворяет требованиям пакета. В сообщении указано невыполненное требование |
| `DownloadFailed` | `Installed`, `UpdateInstalled`, `ConfigurationApplied`, `Managed`, `Ready` | DP не удаётся загрузить версию пакета. Попросите администратора проверить PackageRepository и доступ к хранилищу образов |
| `LoadFromFilesystemFailed` | `Installed`, `UpdateInstalled`, `Ready` | DP не удаётся прочитать загруженный пакет. Обратитесь к разработчику пакета |
| `SettingsInvalid` | `Installed`, `UpdateInstalled`, `ConfigurationApplied` | Настройки не прошли проверку. Сверьте `spec.settings` со схемой настроек в [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) |
| `HookInitializationFailed`, `HookFailed` | `Installed`, `UpdateInstalled`, `ConfigurationApplied`, `Managed`, `Ready` | Хук пакета завершился с ошибкой. Текст ошибки указан в сообщении |
| `ManifestsApplyFailed` | `Installed`, `UpdateInstalled`, `ConfigurationApplied`, `Managed`, `Ready` | DP не удаётся применить манифесты пакета. Проверьте сообщение и события в неймспейсе |
| `ApplyingManifests`, `SettingsChanged` | `UpdateInstalled`, `ConfigurationApplied`, `Managed`, `Ready` | Не ошибка: DP применяет манифесты или изменённые настройки |
| `Reconciling` | `Scaled` | Идёт развёртывание рабочей нагрузки. Её имя указано в сообщении |
| `Degraded` | `Scaled` | Рабочую нагрузку не удалось развернуть. Проверьте события и поды рабочей нагрузки с помощью `d8 k describe` |
| `NoResourceReconciliation` | `Managed` | Приложение находится в режиме обслуживания (поле `spec.maintenance`) |
| `Deleting` | Все | Приложение удаляется |
