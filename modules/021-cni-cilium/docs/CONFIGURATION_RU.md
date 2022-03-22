---
title: "Модуль cni-cilium: настройки"
---

Модуль по умолчанию **выключен**.

Для включения в bare metal, добавьте в ConfigMap `deckhouse`:
```
cniCiliumEnabled: "true"
```

## Параметры

<!-- SCHEMA -->

