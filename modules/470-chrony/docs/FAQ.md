---
title: "The chrony module: FAQ"
type:
  - instruction
---

## How do I disable chrony and use NTP daemon on nodes?

1. [Disable](configuration.html) usage of chrony module.

1. Create `NodeGroupConfiguration` custom step to enable use NTP daemon on nodes (example for `systemd-timesyncd`):

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

   The script removes the boot-time synchronization drop-ins and metric file that remain after disabling the module.
