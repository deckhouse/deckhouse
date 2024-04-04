---
title: Оповещение об обновлении Deckhouse Kubernetes Platform
permalink: ru/update/notifications/dkp-notice/
lang: ru
---

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