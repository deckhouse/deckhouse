---
title: "Модуль deckhouse: настройки"
permalink: /modules/020-deckhouse/configuration.html
---

<!-- SCHEMA -->

**Внимание!** В случае, если в `nodeSelector` указан несуществующий label, или указаны неверные `tolerations`, Deckhouse перестанет работать. Для восстановления работоспособности необходимо изменить значения на правильные в `configmap/deckhouse` и в `deployment/deckhouse`.
