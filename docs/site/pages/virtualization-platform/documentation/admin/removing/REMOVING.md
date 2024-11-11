---
title: "Removing the platform"
permalink: en/virtualization-platform/documentation/admin/removing/removing.html
---

Follow the steps below to delete a cluster that has been manually installed (e.g., on bare metal):

1. [Delete](../modules/040-node-manager/faq.html#how-to-clean-up-a-node-for-adding-to-the-cluster) all the extra nodes from the cluster.

2. Find out the update channel set in the cluster. To do this, run the command:

   ```shell
   kubectl get mc deckhouse  -o jsonpath='{.spec.settings.releaseChannel}'
   ```

3. Run the Deckhouse installer:

   ```shell
   docker run --pull=always -it [<MOUNT_OPTIONS>] \
     registry.deckhouse.io/deckhouse/<DECKHOUSE_REVISION>/install:<RELEASE_CHANNEL> bash
   ```

   where:
   - `<MOUNT_OPTIONS>` — parameters for mounting files in the installer container, such as SSH access keys;
   - `<DECKHOUSE_REVISION>` — the Deckhouse [edition](../revision-comparison.html) (e.g., `ee` — for the Enterprise Edition, `ce` — for the Community Edition, etc.)
   - `<RELEASE_CHANNEL>` — the Deckhouse [release channel](../modules/002-deckhouse/configuration.html#parameters-releasechannel) in kebab-case. Note that it must match the one set in `config.yml`:
     - `alpha` — for the *Alpha* release channel;
     - `beta` — for the *Beta* release channel;
     - `early-access` — for the *Early Access* release channel;
     - `stable` — for the *Stable* release channel;
     - `rock-solid` — for the *Rock Solid* release channel.

   Below is an example command to run the Deckhouse CE installer container:

   ```shell
   docker run -it --pull=always \
     -v "$PWD/dhctl-tmp:/tmp/dhctl" \
     -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.io/deckhouse/ce/install:stable bash
   ```

4. Run the command below to delete the cluster:

   ```shell
   dhctl destroy --ssh-user=<USER> \
     --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
     --yes-i-am-sane-and-i-understand-what-i-am-doing \
     --ssh-host=<MASTER_IP>
   ```

   where:
   - `<USER>` — the user of the remote machine that ran the installation;
   - `<MASTER_IP>` — the IP address of the cluster's master node.

The installer will then connect to the master node and delete all Deckhouse and Kubernetes cluster components on it.