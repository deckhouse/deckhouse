---
title: "Управление control plane: настройки"
---

Управление компонентами control plane кластера осуществляется с помощью модуля `control-plane-manager`, а параметры кластера, влияющие на управление control plane, берутся из Custom Resource `ClusterConfiguration` (создается при инсталляции).

Модуль по умолчанию **включен**. Выключить можно стандартным способом:

```yaml
controlPlaneManagerEnabled: "false"
```

## Параметры

<!-- SCHEMA -->
