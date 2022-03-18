---
title: "Модуль cni-cilium: настройки"
---

Модуль по-умолчанию выключен.
Для включения в bare metal, необходимо в configMap Deckhouse добавить:
```
cniCiliumEnabled: "true"
```

## Параметры

<!-- SCHEMA -->

