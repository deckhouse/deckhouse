---
title: "Модуль cilium-hubble: настройки"
---

Модуль по умолчанию **выключен**.

Для включения, необходимо в ConfigMap `deckhouse` добавить:

```yaml
ciliumHubbleEnabled: "true"
```

Модуль останется отключенным вне зависимости от параметра `ciliumHubbleEnabled:`, если не включен модуль `cni-cilium`. 

## Параметры

<!-- SCHEMA -->
