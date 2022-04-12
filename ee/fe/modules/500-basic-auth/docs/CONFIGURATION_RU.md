---
title: "Модуль basic-auth: настройки"
---

Модуль по умолчанию **выключен**. Для включения добавьте в ConfigMap `deckhouse`:

```yaml
data:
  basicAuthEnabled: "true"
```

Обязательных настроек нет.
По умолчанию создается location `/` с пользователем `admin`.

## Параметры

<!-- SCHEMA -->
