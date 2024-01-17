---
title: "Модуль deckhouse: примеры конфигурации"
---

## Пример конфигурации модуля

Ниже представлен простой пример конфигурации модуля:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    logLevel: Debug
    bundle: Minimal
    releaseChannel: EarlyAccess
```

Также можно настроить дополнительные параметры.

## Настройка режима обновления

Если в автоматическом режиме окна обновлений не заданы, Deckhouse обновится сразу, как только новый релиз станет доступен.

Patch-версии (например, обновления с `1.26.1` до `1.26.2`) устанавливаются без подтверждения и без учета окон обновлений.

{% alert %}
Вы также можете настраивать окна disruption-обновлений узлов в custom resource'ах [NodeGroup](../040-node-manager/cr.html#nodegroup) (параметр `disruptions.automatic.windows`).
{% endalert %}

### Конфигурация окон обновлений

Настроить время, когда Deckhouse будет устанавливать обновления, можно в параметре [update.windows](configuration.html#parameters-update-windows) конфигурации модуля.

Пример настройки двух ежедневных окон обновлений: с 8:00 до 10:00 и c 20:00 до 22:00 (UTC):

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

Также можно настроить обновления в определенные дни, например по вторникам и субботам с 18:00 до 19:30 (UTC):

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

При необходимости возможно включить ручное подтверждение обновлений. Сделать это можно следующим образом:

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

В этом режиме необходимо подтверждать каждое минорное обновление Deckhouse (без учета patch-версий).

Пример подтверждения обновления на версию `v1.43.2`:

```shell
kubectl patch DeckhouseRelease v1.43.2 --type=merge -p='{"approved": true}'
```

### Ручное подтверждение потенциально опасных (disruptive) обновлений

При необходимости возможно включить ручное подтверждение потенциально опасных (disruptive) обновлений (которые меняют значения по умолчанию или поведение некоторых модулей). Сделать это можно следующим образом:

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

В этом режиме необходимо подтверждать каждое минорное потенциально опасное (disruptive) обновление Deckhouse (без учета patch-версий) с помощью аннотации `release.deckhouse.io/disruption-approved=true` на соответствующем ресурсе [DeckhouseRelease](cr.html#deckhouserelease).

Пример подтверждения минорного потенциально опасного обновления Deckhouse `v1.36.4`:

```shell
kubectl annotate DeckhouseRelease v1.36.4 release.deckhouse.io/disruption-approved=true
```

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

После появления новой минорной версии Deckhouse на используемом канале обновлений, но до момента применения ее в кластере на адрес webhook'а будет выполнен [POST-запрос](configuration.html#parameters-update-notification-webhook).

Чтобы всегда иметь достаточно времени для реакции на оповещение об обновлении Deckhouse, достаточно настроить параметр [minimalNotificationTime](configuration.html#parameters-update-notification-minimalnotificationtime). В этом случае обновление случится по прошествии указанного времени с учетом окон обновлений.

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

{% alert %}
Если не указать адрес в параметре [update.notification.webhook](configuration.html#parameters-update-notification-webhook), но указать время в параметре [update.notification.minimalNotificationTime](configuration.html#parameters-update-notification-minimalnotificationtime), применение новой версии все равно будет отложено как минимум на указанное в параметре `minimalNotificationTime` время. В этом случае оповещением о появлении новой версии можно считать появление в кластере ресурса [DeckhouseRelease](cr.html#deckhouserelease), имя которого соответствует новой версии.
{% endalert %}

## Сбор информации для отладки

О сборе отладочной информации читайте [в FAQ](faq.html#как-собрать-информацию-для-отладки).
