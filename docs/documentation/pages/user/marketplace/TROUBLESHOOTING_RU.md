---
title: Диагностика
permalink: ru/user/marketplace/troubleshooting.html
description: "Диагностика и устранение проблем с приложениями Deckhouse Kubernetes Platform Marketplace. Проверка наличия CRD, чтение условий и summary приложения, просмотр логов."
lang: ru
search: Application troubleshooting, application conditions, диагностика приложений, условия приложения, логи приложения
---

## Проверка наличия CRD Marketplace

Если `d8 k get app` возвращает ошибку, возможно, CRD Marketplace не установлены. Для проверки выполните следующую команду:

```bash
d8 k get crd | grep -E 'application|package'
```

Ожидаемый вывод:

<!-- markdownlint-disable MD031 -->
```console
applicationpackages.deckhouse.io                     2026-02-10T14:54:41Z
applicationpackageversions.deckhouse.io              2026-02-10T14:54:41Z
applications.deckhouse.io                            2026-02-10T14:54:41Z
packagerepositories.deckhouse.io                     2026-02-10T14:54:41Z
packagerepositoryoperations.deckhouse.io             2026-02-10T14:54:41Z
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

Если какие-либо CRD отсутствуют, обратитесь к администратору кластера. Marketplace требует DKP версии 1.76 или выше.

## Чтение summary приложения

Самый быстрый способ понять, почему приложение не работает — проверить `status.summary`. Для этого выполните следующую команду (можно использовать сокращённое имя — `app`):

```bash
d8 k get app -n <NAMESPACE> <APPLICATION_NAME> -o yaml | grep -A5 'summary:'
```

Пример вывода:

```yaml
summary:
  state: Updating
  message: "Update is waiting for dependent modules to converge; previous version is still serving"
  tip: "Waiting until DKP processes all dependent modules to start the update."
```

- **`state`** — текущее общее состояние приложения.
- **`message`** — объясняет, почему приложение находится в этом состоянии.
- **`tip`** — что нужно сделать для решения проблемы или чего ожидает DKP.

## Чтение отдельных условий

Для более детального просмотра состояния выполните следующую команду:

```bash
d8 k get app -n <NAMESPACE> <APPLICATION_NAME> \
  -o jsonpath='{range .status.conditions[*]}{.type}: {.status} ({.reason}) - {.message}{"\n"}{end}'
```

Пример вывода при зависшем обновлении:

```yaml
conditions:
  - lastTransitionTime: "2026-02-25T16:39:30Z"
    message: ""
    observedGeneration: 1
    reason: Installed
    status: "True"
    type: Installed
  - lastTransitionTime: "2026-02-25T17:12:25Z"
    message: "Update is waiting for dependent modules to converge"
    observedGeneration: 1
    reason: Pending
    status: "False"
    type: UpdateInstalled
  - lastTransitionTime: "2026-02-25T16:39:30Z"
    message: ""
    observedGeneration: 1
    reason: ConfigurationApplied
    status: "True"
    type: ConfigurationApplied
  - lastTransitionTime: "2026-02-25T16:39:30Z"
    message: ""
    observedGeneration: 1
    reason: Managed
    status: "True"
    type: Managed
  - lastTransitionTime: "2026-02-25T16:39:30Z"
    message: ""
    observedGeneration: 1
    reason: Scaled
    status: "True"
    type: Scaled
  - lastTransitionTime: "2026-02-25T16:39:30Z"
    message: ""
    observedGeneration: 1
    reason: Ready
    status: "True"
    type: Ready
currentVersion:
  version: v0.0.20
```

В этом примере `Installed=True` (приложение запущено на версии v0.0.20), но `UpdateInstalled=False/Pending` означает, что обновление в очереди и ожидает завершения зависимого модуля.

## Просмотр логов контроллера DKP

Если условий статуса недостаточно для диагностики, просмотрите логи контроллера:

```bash
d8 k logs deployments/deckhouse -n d8-system | grep <APPLICATION_NAME>
```

## Просмотр логов подов приложения

Для получения списка подов, созданных приложением, выполните следующую команду:

```bash
d8 k get pods -n <NAMESPACE> -l app.kubernetes.io/instance=<APPLICATION_NAME>
```

Для просмотра логов конкретного пода выполните:

```bash
d8 k logs -n <NAMESPACE> <POD_NAME>
```

Для просмотра логов конкретного деплоймента с префиксом имени экземпляра выполните:

```bash
d8 k logs -n <NAMESPACE> deployments/<APPLICATION_NAME>-<RESOURCE_NAME>
```

## Частые условия и их значение

| Условие | Reason при Status=False | Что проверить |
|---|---|---|
| `Installed` | `InstallFailed` | Логи контроллера DKP, проверьте настройки по OpenAPI-схеме |
| `UpdateInstalled` | `Pending` | Конвергенция зависимых модулей — проверьте условия модуля `d8` |
| `UpdateInstalled` | `UpdateFailed` | Указанная `packageVersion` не существует в репозитории — проверьте через `d8 k get apv -l package=<имя>` |
| `ConfigurationApplied` | `ConfigurationFailed` | Ошибка валидации настроек — проверьте по схеме [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) |
| `Scaled` | `NotScaled` | Поды не готовы — проверьте события пода через `d8 k describe pod` |
| `Ready` | `NotReady` | Одно или несколько условий выше не выполнены |
