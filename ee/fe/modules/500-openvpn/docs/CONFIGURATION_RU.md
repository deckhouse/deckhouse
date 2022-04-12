---
title: "Модуль openvpn: настройки"
---

Модуль по умолчанию **выключен**. Для включения добавьте в ConfigMap `deckhouse`:

```yaml
data:
  openvpnEnabled: "true"
```

## Параметры

<!-- SCHEMA -->
