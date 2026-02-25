---
title: Как узнать, что для кластера доступна новая версия DKP?
subsystems:
  - deckhouse
lang: ru
---

Как только на установленном в кластере канале обновления появляется новая версия DKP:

- Загорается [алерт `DeckhouseReleaseIsWaitingManualApproval`](../../../reference/alerts.html#monitoring-deckhouse-deckhousereleaseiswaitingmanualapproval), если кластер использует [ручной режим обновлений](configuration.html#ручное-подтверждение-обновлений).
- Появляется новый кастомный ресурс [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease). Используйте команду `d8 k get deckhousereleases`, чтобы посмотреть список релизов. Если `DeckhouseRelease` новой версии находится в состоянии `Pending`, указанная версия еще не установлена. Возможные причины, при которых `DeckhouseRelease` может находиться в `Pending`:
  - Установлен [ручной режим обновлений](configuration.html#ручное-подтверждение-обновлений).
  - Установлен автоматический режим обновлений и настроены [окна обновлений](configuration.html#окна-обновлений), интервал которых еще не наступил.
  - Установлен автоматический режим обновлений, окна обновлений не настроены, но применение версии отложено на случайный период времени из-за механизма снижения нагрузки на репозиторий образов контейнеров. В поле `status.message` ресурса DeckhouseRelease будет соответствующее сообщение.
  - Установлен [параметр `update.notification.minimalNotificationTime`](/modules/deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime) и заданный в нем интервал еще не прошел.
