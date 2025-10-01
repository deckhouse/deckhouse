---
title: Synchronizing time on nodes
permalink: en/admin/configuration/network/other/ntp.html
---

To synchronize time on Deckhouse cluster nodes,
DKP uses a built-in solution based on [chrony](https://chrony-project.org/).
Using the Network Time Protocol (NTP),
DKP ensures that system clocks on cluster nodes are synchronized with external NTP servers.
If required, you can disable this built-in mechanism and configure custom NTP daemons.

## Enabling built-in time synchronization

Enable the [`chrony`](/modules/chrony/) module to activate time synchronization:

```shell  
d8 platform module enable chrony
```

By default, the time source is the server `pool.ntp.org`.
You can specify a list of NTP servers using the [`ntpServers`](/modules/chrony/configuration.html#parameters-ntpservers) parameter
in the configuration of the `chrony` module.

An example of the module configuration specifying NTP servers:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: chrony
spec:
  enabled: true
  settings:
    ntpServers:
      - ntp.ubuntu.com
      - time.google.com
  version: 1
```

## Using custom NTP daemons

To disable the built-in time synchronization mechanism and use your own NTP daemons on the nodes, follow these steps:

1. Disable the [`chrony`](/modules/chrony/) module:

   ```shell
   d8 platform module disable chrony
   ```

   If the command is successful, you should see the message confirming that the module has been disabled:

   ```console
   Module chrony disabled
   ```

1. Create a [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) resource
   to enable the NTP daemons on the nodes.

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
