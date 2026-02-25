---
title: Как понять, что в кластере идет обновление?
subsystems:
  - deckhouse
lang: ru
---

Во время обновления:

- отображается [алерт `DeckhouseUpdating`](../../../reference/alerts.html#monitoring-deckhouse-deckhouseupdating);
- под `deckhouse` находится не в статусе `Ready`. Если под долго не переходит в статус `Ready`, это может говорить о наличии проблем в работе DKP. Необходима диагностика.
