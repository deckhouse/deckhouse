---
title: "Модуль cilium-hubble: настройки"
---

Модуль включается **автоматически** если включен `cni-cilium` модуль.
Для выключения, необходимо в ConfigMap `deckhouse` добавить:
```
ciliumHubbleEnabled: "false"
```

## Параметры

<!-- SCHEMA -->

