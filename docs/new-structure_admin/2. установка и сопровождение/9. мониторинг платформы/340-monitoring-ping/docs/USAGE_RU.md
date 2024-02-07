---
title: "Модуль monitoring-ping: примеры конфигурации"
---

## Добавление дополнительных IP-адресов для мониторинга

Для добавления дополнительных IP-адресов мониторинга используйте параметр [externalTargets](configuration.html#parameters-externaltargets) модуля.

Пример конфигурации модуля:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: monitoring-ping
spec:
  version: 1
  enabled: true
  settings:
    externalTargets:
    - name: google-primary
      host: 8.8.8.8
    - name: yaru
      host: ya.ru
    - host: youtube.com
```

> Поле `name` используется в Grafana для отображения связанных данных. Если поле `name` не указано, используется обязательное поле `host`.
