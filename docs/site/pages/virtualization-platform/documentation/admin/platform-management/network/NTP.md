---
title: "NTP management"
permalink: en/virtualization-platform/documentation/admin/platform-management/network/ntp.html
---

## Configuring node time synchronization

To configure time synchronization on nodes, use the chrony module or replace it with a custom NTP daemon.

To enable the kube-dns module with default settings,
apply the `ModuleConfig` resource, specifying your NTP servers for synchronization.
Example of configuration with a default NTP server:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: chrony
spec:
  enabled: true
  settings:
    ntpServers:
      - pool.ntp.org
  version: 1
EOF
```

## Using NTP daemons

To disable chrony and use custom NTP daemons on nodes, disable the module:

```shell
d8 k -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module disable chrony

# Module chrony disabled
```

To enable NTP daemons on nodes, create [NodeGroupConfiguration](todo).
Below is an example configuration using systemd-timesyncd:

```yaml
d8 k apply -f - <<EOF
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
EOF
```
