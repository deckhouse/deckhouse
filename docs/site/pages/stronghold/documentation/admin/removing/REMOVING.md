---
title: "Removing the platform"
permalink: en/stronghold/documentation/admin/removing/removing.html
---

To delete a cluster, several steps need to be followed:

1. Remove all additional nodes from the cluster:

   1.1. Remove the node from the Kubernetes cluster:

   ```shell
   d8 k drain <node> --ignore-daemonsets --delete-local-data
   d8 k delete node <node>
   ```

   1.2. Run the cleanup script on the node:

   ```shell
   bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
   ```

1. Check the release channel set in the cluster. To do this, run the command:

   ```shell
   d8 k get mc deckhouse -o jsonpath='{.spec.settings.releaseChannel}'
   ```

1. Run the Deckhouse installer:

   ```shell
   docker run --pull=always -it [<MOUNT_OPTIONS>] \
      registry.deckhouse.io/deckhouse/<DECKHOUSE_REVISION>/install:<RELEASE_CHANNEL> bash
   ```

   where:
   - `<MOUNT_OPTIONS>` — the options for mounting files into the installer container, such as SSH access keys;
   - `<DECKHOUSE_REVISION>` — [edition](../../about/editions.html) of the platform (e.g., `ee` — for Enterprise Edition, `ce` — for Community Edition, etc.)
   - `<RELEASE_CHANNEL>` — [release channel](../../about/release-channels.html) of the platform in kebab-case. It should match the one set in `config.yaml` during the [installation](../install/steps/install.html):
     - `alpha` — for the *Alpha* release channel;
     - `beta` — for the *Beta* release channel;
     - `early-access` — for the *Early Access* release channel;
     - `stable` — for the *Stable* release channel;
     - `rock-solid` — for the *Rock Solid* release channel.

   Example of running the installer container for the CE edition:

   ```shell
   docker run -it --pull=always \
      -v "$PWD/dhctl-tmp:/tmp/dhctl" \
      -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.io/deckhouse/ce/install:stable bash
   ```

1. Execute the cluster removal command:

   ```shell
   dhctl destroy --ssh-user=<USER> \
      --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
      --yes-i-am-sane-and-i-understand-what-i-am-doing \
      --ssh-host=<MASTER_IP>
   ```

   where:
   - `<USER>` — the user of the remote machine from which the installation was performed. If `<USER>` needs to perform actions via `sudo`, add the `--ask-become-pass` flag;
   - `<MASTER_IP>` — IP address of the master node in the cluster.

   The installer will connect to the master node and remove all Deckhouse components and the Kubernetes cluster from it.
