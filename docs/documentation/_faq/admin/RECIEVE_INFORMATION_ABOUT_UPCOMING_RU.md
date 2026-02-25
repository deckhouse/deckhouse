---
title: Как заранее получать информацию о предстоящем обновлении?
subsystems:
  - deckhouse
lang: ru
---

Получать заранее информацию об обновлении минорных версий DKP на канале обновлений можно следующими способами:

- Настроить [ручной режим обновлений](configuration.html#ручное-подтверждение-обновлений). В этом случае при появлении новой версии на канале обновлений загорится [алерт `DeckhouseReleaseIsWaitingManualApproval`](../../../reference/alerts.html#monitoring-deckhouse-deckhousereleaseiswaitingmanualapproval), и в кластере появится новый [кастомный ресурс DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease).
- Настроить [автоматический режим обновлений](configuration.html#автоматическое-обновление) и указать минимальное время в [параметре `minimalNotificationTime`](/modules/deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime), на которое будет отложено обновление. В этом случае при появлении новой версии на канале обновлений в кластере появится новый [кастомный ресурс DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease). Если указать URL в параметре [`update.notification.webhook`](/modules/deckhouse/configuration.html#parameters-update-notification-webhook), через указанный вебхук будет отправлено уведомление об предстоящем обновлении.
