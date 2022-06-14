---
title: "The chrony module: FAQ"
type:
  - instruction
---

## How do I disable chrony and use ntp daemon on nodes?

1. Disable usage of chrony module in Deckhouse CM:

```yaml
chronyEnabled: "false"
```

2. Create NodeGroupConfiguration custom step to enable use ntp daemon on nodes (example for systemd-timesyncd):

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
