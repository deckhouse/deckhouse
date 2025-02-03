---
title: "Модуль cni-simple-bridge: настройки"
description: "Настройка модуля cni-simple-bridge"
---

Модуль не имеет настроек, но для его использования необходимо явным образом включить его через `ModuleConfig`.

Для явного включения или отключения модуля необходимо установить `true` или `false` в поле `.spec.enabled` в соответствующем кастомном ресурсе `ModuleConfig`. Если для модуля нет такого кастомного ресурса `ModuleConfig`, его нужно создать.

## Пример конфигурации

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-simple-bridge
spec:
  enabled: true
  version: 1
```

<!-- SCHEMA -->
