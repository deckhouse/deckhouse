---
title: "Модуль chrony: FAQ"
type:
  - instruction
---

## Как запретить использование chrony и использовать ntp-демоны на узлах?

1. Выключите модуль chrony в Deckhouse CM:

```yaml
chronyEnabled: "false"
```

2. Создайте NodeGroupConfiguration custom step чтобы включить ntp-демоны на узлах (пример для systemd-timesyncd):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: enable_ntp_on_node.sh
spec:
  weight: 100
  nodeGroups: ["*"]
  bundles: ["*"]
  content: |
    systemctl enable systemd-timesyncd
    systemctl start systemd-timesyncd
```
