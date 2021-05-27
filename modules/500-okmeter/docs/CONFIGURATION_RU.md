---
title: "Модуль okmeter: настройки"
---

Модуль по умолчанию **выключен**. Для включения добавьте в CM `deckhouse`:

```yaml
data:
  okmeterEnabled: "true"
```

В конфигурацию Deckhouse необходимо добавить `apiKey` для модуля `okmeter`:

* `apiKey` - этот ключ можно взять на странице документации по установке `okmeter` для нужного проекта (`OKMETER_API_TOKEN`).
