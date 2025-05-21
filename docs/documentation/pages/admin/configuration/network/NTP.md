---
title: Synchronizing time on nodes
permalink: en/admin/network/ntp.html
---

To synchronize time on Deckhouse cluster nodes,
DKP uses a built-in solution based on [chrony](https://chrony-project.org/).
Using the Network Time Protocol (NTP),
DKP ensures that system clocks on cluster nodes are synchronized with external NTP servers.
If required, you can disable this built-in mechanism and configure custom NTP daemons.

## Enabling built-in time synchronization

To enable time synchronization with default settings, apply a ModuleConfig resource specifying the list of NTP servers.
Example configuration using `pool.ntp.org`:

```shell
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

## Using custom NTP daemons

1. To disable the built-in time synchronization mechanism and use your own NTP daemons on the nodes,
   disable the `chrony` module:

   ```shell
   d8 k -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module disable chrony
   ```

   If the command is successful, you should see the message confirming that the module has been disabled:

   ```console
   Module chrony disabled
   ```

1. Create a [NodeGroupConfiguration](../../reference/cr/nodegroupconfiguration.html) resource to enable the NTP daemons on the nodes.
   Example for `systemd-timesyncd`:

   ```shell
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
