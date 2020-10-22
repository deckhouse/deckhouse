---
title: "Модуль monitoring-ping: примеры конфигурации"
---

## Добавление дополнительных IP адресов для мониторинга

В конфигурацию модуля в ConfigMap deckhouse можно разместить подобную конструкцию:
```yaml
  monitoringPing: |
    externalTargets:
    - name: google-primary
      host: 8.8.8.8
    - name: yaru
      host: ya.ru
    - host: youtube.com
```

Поле `name` используется для отображения в графане. Если не указать `name`, то используется обязательное поле `host`.
