---
title: "Модуль deckhouse: примеры конфигурации"
---

## Пример конфигурации модуля

Пример конфигурации модуля с настройкой автоматического обновления Deckhouse на канал обновлений EarlyAccess:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: EarlyAccess
```

## Настройка режима обновления

Если в автоматическом режиме окна обновлений не заданы, Deckhouse обновится сразу, как только новый релиз станет доступен.

Patch-версии (например, обновления с `1.26.1` до `1.26.2`) устанавливаются без подтверждения и без учета окон обновлений.

> Вы также можете настроить окна disruption-обновлений узлов с помощью параметра [`disruptions.automatic.windows`](../040-node-manager/cr.html#nodegroup-v1-spec-disruptions-automatic-windows) ресурса `NodeGroup`.

### Конфигурация окон обновлений

Настроить время, когда Deckhouse будет устанавливать обновления, можно в параметре [update.windows](configuration.html#parameters-update-windows) конфигурации модуля.

Пример настройки двух ежедневных окон обновлений: с 8 до 10 и c 20 до 22 (UTC):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: EarlyAccess
    update:
      windows: 
        - from: "8:00"
          to: "10:00"
        - from: "20:00"
          to: "22:00"
```

Также можно настроить обновления в определенные дни, например, по вторникам и субботам с 13:00 до 18:30 (UTC):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      windows: 
        - from: "18:00"
          to: "19:30"
          days:
            - Tue
            - Sat
```

### Ручное подтверждение обновлений

Ручное подтверждение обновлений можно включить с помощью параметра [update.mode](configuration.html#parameters-update-mode) модуля. В этом режиме будет необходимо подтверждать каждое **минорное** обновление Deckhouse. Обновление patch-версии Deckhouse в этом режиме будет выполняться автоматически, без подтверждений.

Пример конфигурации модуля (включение канала обновлений Stable, с ручным режимом обновления):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      mode: Manual
```

Для подтверждения обновления в соответствующем custom resource [`DeckhouseRelease`](cr.html#deckhouserelease) поле `approved` необходимо установить в `true`.

Пример подтверждения обновления на версию `v1.43.2`:

```shell
kubectl patch DeckhouseRelease v1.43.2 --type=merge -p='{"approved": true}'
```

### Ручное подтверждение потенциально опасных (disruptive) обновлений

{% alert %}
**Потенциально опасное обновление (disruptive-обновление)** может привести к временному прерыванию работы важного компонента кластера, пользовательского приложения или связанных систем. Такое обновление, например, может переопределить значение по умолчанию или изменить поведение некоторых модулей.
{% endalert %}

Включить ручное подтверждение _потенциально опасных обновлений_ можно с помощью параметра [update.disruptionApprovalMode](configuration.html#parameters-update-disruptionapprovalmode). Пример конфигурации:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      disruptionApprovalMode: Manual
```

В этом режиме необходимо подтверждать каждое минорное потенциально опасное (disruptive) обновление Deckhouse (без учёта patch-версий) с помощью аннотации `release.deckhouse.io/disruption-approved=true` на соответствующем ресурсе [`DeckhouseRelease`](cr.html#deckhouserelease). Обычное обновление (не потенциально опасное) будет применяться автоматически.

Пример подтверждения минорного потенциально опасного обновления Deckhouse `v1.36.4`:

```shell
kubectl annotate DeckhouseRelease v1.36.4 release.deckhouse.io/disruption-approved=true
```

{% alert level="warning" %}
Параметр [disruptionApprovalMode](configuration.html#parameters-update-disruptionapprovalmode) не влияет на режим обновления кластера (параметр [update.mode](configuration.html#parameters-update-mode)). Например, при следующей конфигурации Deckhouse будет обновляться автоматически в пределах установленного окна обновлений по понедельникам и вторникам с 10 до 13 UTC, но не будет обновляться на версии, которые помечены как потенциально опасные:

```yaml
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      disruptionApprovalMode: Manual
      windows:
      - days:
        - Mon
        - Tue
        from: "10:00"
        to: "13:00"
```

{% endalert %}

### Оповещение об обновлении Deckhouse

В режиме обновлений `Auto` можно [настроить](configuration.html#parameters-update-notification) вызов webhook'а для получения оповещения о предстоящем обновлении минорной версии Deckhouse.

Пример настройки оповещения:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    update:
      releaseChannel: Stable
      mode: Auto
      notification:
        webhook: https://release-webhook.mydomain.com
```

После появления новой минорной версии Deckhouse на используемом канале обновлений, но до момента применения ее в кластере, на адрес webhook'а будет выполнен [POST-запрос](configuration.html#parameters-update-notification-webhook).

Чтобы всегда иметь достаточно времени для реакции на оповещение об обновлении Deckhouse, достаточно настроить параметр [minimalNotificationTime](configuration.html#parameters-update-notification-minimalnotificationtime). В этом случае обновление случится по прошествии указанного времени, с учетом окон обновлений.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    update:
      releaseChannel: Stable
      mode: Auto
      notification:
        webhook: https://release-webhook.mydomain.com
        minimalNotificationTime: 8h
```

## Сбор информации для отладки

О сборе отладочной информации читайте [в FAQ](faq.html#как-собрать-информацию-для-отладки).
