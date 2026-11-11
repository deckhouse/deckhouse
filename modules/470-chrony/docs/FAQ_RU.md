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
       if [ -f /etc/systemd/system/kubelet.service.d/20-d8-early-time-sync.conf ] || [ -f /etc/systemd/system/nodelet.service.d/20-d8-early-time-sync.conf ]; then
         rm -f /etc/systemd/system/kubelet.service.d/20-d8-early-time-sync.conf
         rm -f /etc/systemd/system/nodelet.service.d/20-d8-early-time-sync.conf
         systemctl daemon-reload
       fi
       rm -f /var/run/node-exporter-textfile/d8_early_time_sync.prom
       systemctl enable systemd-timesyncd
       systemctl start systemd-timesyncd
   ```

   Скрипт удаляет drop-in-файлы ранней синхронизации времени и файл метрики, которые остаются на узлах после отключения модуля.
