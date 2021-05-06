---
title: "Управление control plane: настройки"
---

Управление компонентами control plane кластера осуществляется с помощью модуля `control-plane-manager`, а параметры кластера, влияющие на управление control plane, берутся из данных первичной конфигурации кластера (параметр `cluster-configuration.yaml` секрета `d8-cluster-configuration` в namespace `kube-system`), которая создается при инсталляции.

Модуль по умолчанию **включен**. Выключить можно стандартным способом:

```yaml
controlPlaneManagerEnabled: "false"
```

## Параметры

<!-- SCHEMA -->
