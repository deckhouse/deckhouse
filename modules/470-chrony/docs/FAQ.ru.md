---
title: "Модуль chrony: FAQ"
type:
  - instruction
---

## Как запретить использование chrony и использовать NTP-демоны на узлах?

1. [Выключите](configuration.html) модуль chrony.

1. Создайте `NodeGroupConfiguration` custom step, чтобы включить NTP-демоны на узлах (пример для `systemd-timesyncd`):

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
