---
title: "Обновление платформы"
permalink: ru/virtualization-platform/documentation/admin/update/update.html
lang: ru
---

Обновление платформы конфигурируется в ресурсе ModuleConfig [`deckhouse`](../../reference/configuration-module.html). Todo.

Посмотреть текущую конфигурацию обновления можно с помощью команды:

```shell
d8 k get mc deckhouse -oyaml
```

Пример вывода:

```yaml
...
spec:
  settings:
    releaseChannel: Stable
    update:
      windows:
        - days:
            - Mon
          from: "19:00"
          to: "20:00"
...
```

## Настройка режима обновления

Платформа поддерживает три режима обновления:

- **Автоматический + окна обновлений не заданы.** Кластер обновится сразу после появления новой версии на соответствующем [канале обновлений](../update-channels.html).
- **Автоматический + заданы окна обновлений.** Кластер обновится в ближайшее доступное окно после появления новой версии на канале обновлений.
- **Ручной режим.** Для применения обновления требуются [ручные действия](./manual-update-mode.html).

Пример фрагмента конфигурации для включения автоматического обновления платформы:

```yaml
update:
  mode: Auto
```

Пример фрагмента конфигурации для включения автоматического обновления платформы с окнами обновлений:

```yaml
update:
  mode: Auto
  windows:
    - from: "8:00"
      to: "15:00"
      days:
        - Tue
        - Sat
```

Пример фрагмента конфигурации для включения ручного режима обновления платформы:

```yaml
update:
  mode: Manual
```

## Каналы обновлений

Платформа использует [пять каналов обновлений](../update-channels.html), предназначенных для использования в разных окружениях. Компоненты платформы могут обновляться автоматически, либо с ручным подтверждением по мере выхода обновлений в каналах обновления.

Информацию по версиям, доступных на каналах обновления, можно получить на сайте https://releases.deckhouse.ru/

Чтобы перейти на другой канал обновлений, нужно в конфигурации модуля `deckhouse` изменить (установить) параметр `.spec.settings.releaseChannel`.

Пример конфигурации модуля `deckhouse` с установленным каналом обновлений `Stable`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
```

- При смене канала обновлений на **более стабильный** (например, с `Alpha` на `EarlyAccess`) Deckhouse скачивает данные о релизе (в примере — из канала `EarlyAccess`) и сравнивает их с данными из существующих в кластере custom resource'ов `DeckhouseRelease`:
  - Более _поздние_ релизы, которые еще не были применены (в статусе `Pending`), удаляются.
  - Если более _поздние_ релизы уже применены (в статусе `Deployed`), смены релиза не происходит. В этом случае Deckhouse останется на таком релизе до тех пор, пока на канале обновлений `EarlyAccess` не появится более поздний релиз.
- При смене канала обновлений на **менее стабильный** (например, с `EarlyAcess` на `Alpha`):
  - Deckhouse скачивает данные о релизе (в примере — из канала `Alpha`) и сравнивает их с данными из существующих в кластере custom resource'ов `DeckhouseRelease`.
  - Затем Deckhouse выполняет обновление согласно установленным параметрам обновления.

Посмотреть список релизов платформы можно с использованием следующих команд:

```shell
d8 k get deckhouserelease
d8 k get modulereleases
```
{% offtopic title="Схема использования параметра releaseChannel при установке и в процессе работы Deckhouse" %}

![Схема использования параметра releaseChannel при установке и в процессе работы Deckhouse](../../../../../../../docs/documentation/images/common/deckhouse-update-process.png)

{% endofftopic %}

Для отключения механизма обновления Deckhouse, удалите в конфигурации модуля `deckhouse` параметр `.spec.settings.releaseChannel`. В этом случае Deckhouse не проверяет обновления и даже обновление на patch-релизы не выполняется.

{% alert level="danger" %}
Крайне не рекомендуется отключать автоматическое обновление! Это заблокирует обновления на patch-релизы, которые могут содержать исправления критических уязвимостей и ошибок.
{% endalert %}

## Немедленное применение обновлений

Чтобы применить обновление немедленно, установите в соответствующем ресурсе [DeckhouseRelease](../../../../../#deckhouserelease) аннотацию `release.deckhouse.io/apply-now: "true"`.

{% alert level="info" %}
**Обратите внимание!** В этом случае будут проигнорированы окна обновления, настройки [canary-release](../../../../../cr.html#deckhouserelease-v1alpha1-spec-applyafter) и режим [ручного обновления кластера](../../../../../modules/002-deckhouse/configuration.html#parameters-update-disruptionapprovalmode). Обновление применится сразу после установки аннотации.
{% endalert %}

Пример команды установки аннотации пропуска окон обновлений для версии `v1.56.2`:

```shell
d8 k annotate deckhousereleases v1.56.2 release.deckhouse.io/apply-now="true"
```

Пример ресурса с установленной аннотацией пропуска окон обновлений:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  annotations:
    release.deckhouse.io/apply-now: "true"
```