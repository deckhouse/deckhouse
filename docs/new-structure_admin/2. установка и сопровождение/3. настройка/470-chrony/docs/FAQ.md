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
       systemctl enable systemd-timesyncd
       systemctl start systemd-timesyncd
   ```
