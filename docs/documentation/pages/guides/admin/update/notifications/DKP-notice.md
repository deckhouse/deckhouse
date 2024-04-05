---
title: Оповещение об обновлении Deckhouse Kubernetes Platform
permalink: ru/update/notifications/dkp-notice/
lang: ru
---

В режиме обновлений `Auto` можно [настроить](configuration.html#parameters-update-notification) вызов вебхука для получения оповещения о предстоящем обновлении минорной версии Deckhouse.

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

После появления новой минорной версии Deckhouse Kubernetes Platform на используемом канале обновлений, на адрес вебхука будет выполнен [POST-запрос](configuration.html#parameters-update-notification-webhook).

Чтобы постоянно иметь время для реакции на оповещение об обновлении Deckhouse Kubernetes Platform, настройте параметр [minimalNotificationTime](configuration.html#parameters-update-notification-minimalnotificationtime), как показано на примере ниже. В этом случае обновление случится по прошествии указанного времени с учетом окон обновлений.

Пример настройки параметра `minimalNotificationTime`:

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
Если не указать адрес в параметре [update.notification.webhook](configuration.html#parameters-update-notification-webhook), но указать время в параметре [update.notification.minimalNotificationTime](configuration.html#parameters-update-notification-minimalnotificationtime), применение новой версии будет отложено на указанное в параметре `minimalNotificationTime` время. В этом случае оповещением о появлении новой версии ,eltn cxbnfnmcz - появление в кластере ресурса [DeckhouseRelease](cr.html#deckhouserelease), имя которого соответствует новой версии.
{% endalert %}