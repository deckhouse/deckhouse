---
title: "Модуль chrony: FAQ"
type:
  - instruction
---

## Как запретить использование chrony и использовать ntp-демоны на узлах?

1. Выключите модуль chrony в ConfigMap `deckhouse`:

   ```yaml
   chronyEnabled: "false"
   ```

2. Создайте `NodeGroupConfiguration` custom step чтобы включить NTP-демоны на узлах (пример для `systemd-timesyncd`):

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: enable-ntp-on-node.sh
   spec:
     weight: 100
     nodeGroups: ["*"]
     bundles: ["*"]
     content: |
       systemctl enable systemd-timesyncd
       systemctl start systemd-timesyncd
   ```
