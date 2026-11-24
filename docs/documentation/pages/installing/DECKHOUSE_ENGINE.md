---
title: Installing a bare-metal cluster on Deckhouse Engine
permalink: en/installing/deckhouse-engine.html
description: Install a Deckhouse Platform cluster with three Deckhouse Engine master nodes and add a worker node on bare metal.
---

Install Deckhouse Platform on machines running [Deckhouse Engine](../reference/deckhouse-engine.html). The example uses three master nodes and one worker node.

## Before you begin

Prepare four x86-64 machines that boot via Unified Extensible Firmware Interface (UEFI). Each machine needs a blank system disk of at least 20 GiB. In this example, each master node also has a separate blank 10 GiB disk for etcd. The installer formats the selected disks.

{% alert level="info" %}
Use the [cluster hardware requirements](../guides/hardware-requirements.html) to size and select machines for your expected workload.
{% endalert %}

The installer host must be able to reach TCP port `50000` on all three master machines and TCP port `50001` on the first master machine. See the [network requirements](../reference/network_interaction.html) for details.

Install Docker and `jq` on the installer host.

## Preparing the machines

Run the Deckhouse Platform installer for your chosen edition. See the [edition comparison](../reference/revision-comparison.html) for details.

1. Get the digest of the image containing the installation ISO. Use the installer image for the edition and release channel you plan to install:

   ```shell
   docker run --rm --pull=always 'registry.deckhouse.io/deckhouse/<EDITION>/install:<CHANNEL>' \
     cat /deckhouse/candi/images_digests.json | jq -r .nodeManager.engineMetalIso
   ```

   Replace `<EDITION>` with the edition and `<CHANNEL>` with the release channel.

1. Extract the ISO file from the image:

   ```shell
   docker create --name engine-iso 'registry.deckhouse.io/deckhouse/<EDITION>@<ISO_DIGEST>' true
   docker cp engine-iso:/disk/metal.iso .
   docker rm engine-iso
   ```

1. Boot each machine from `metal.iso` using a USB drive or a virtual CD-ROM. The installer gets an address via Dynamic Host Configuration Protocol (DHCP) and waits for configuration. If DHCP is unavailable, configure the address manually.

1. Check that the machine is waiting for configuration:

   ```shell
   curl -s 'http://<NODE_ADDRESS>:50000/whoami'
   ```

   An `installer` response means the machine is ready. An `agent` response means the node is already installed and the machine will reject the configuration.

1. List the machine's disks to identify the system disk:

   ```shell
   curl -s 'http://<NODE_ADDRESS>:50000/inventory'
   ```

   Use `wwid`, `serial`, or `busPath` to select the disk. These attributes remain stable across reboots.

## Installing the cluster

Set [`spec.systemType`](/modules/node-manager/cr.html#nodegroup-v1-spec-systemtype) to `DeckhouseEngine` in the `master` and `worker` NodeGroups. The installer uses the `master` group to configure all three master nodes and creates the `worker` group for later use.

Save the installation configuration as `config.yml`. Set the network ranges and domain template for your cluster.

{% offtopic title="Installation configuration" %}

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "cluster.local"
---
apiVersion: deckhouse.io/v1
kind: StaticClusterConfiguration
internalNetworkCIDRs:
  - 192.168.1.0/24
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    bundle: Default
    releaseChannel: Stable
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings:
    modules:
      publicDomainTemplate: "%s.example.com"
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-cilium
spec:
  version: 1
  enabled: true
  settings:
    tunnelMode: VXLAN
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Static
  systemType: DeckhouseEngine
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  systemType: DeckhouseEngine
---
# Configuration for the first master node. Configure the other master nodes the same way.
apiVersion: internal.deckhouse.io/v1alpha1
kind: NodeConfig
metadata:
  name: master-0
spec:
  storage:
    diskSelector:
      serial: S3Z8NB0K700001
    mounts:
      # The node formats this blank disk for etcd and mounts it at /var/lib/etcd.
      - name: kubernetes-data
        partitionSelector:
          blank: true
          size: ">=10Gi"
```

{% endofftopic %}

If you omit `diskSelector`, the installer selects the system disk by size (`size: ">=20Gi"`).

1. Create the directory for installation logs and kubeconfig, then start the installer container:

   ```shell
   docker run -it --pull=always \
     -v "$PWD/config.yml:/config.yml" \
     -v "$PWD/dhctl-tmp:/tmp/dhctl" \
     'registry.deckhouse.io/deckhouse/<EDITION>/install:<CHANNEL>' bash
   ```

1. In the container, run `dhctl bootstrap`. Replace the example IP addresses with the addresses of the three master machines. Installation starts on the machine listed in the first `--master-host` parameter. In this example, that is `master-0`.

   ```shell
   dhctl bootstrap \
     --config=/config.yml \
     --master-host master-0=192.168.1.10 \
     --master-host master-1=192.168.1.11 \
     --master-host master-2=192.168.1.12
   ```

   The installer connects to Deckhouse Engine on port `50000`. Leave the SSH connection parameters unset.

After installation, the installer retrieves the kubeconfig file from the master node and prints its path in the installation log. Use this file to connect to the Kubernetes API.

{% alert level="warning" %}
Deckhouse Engine master nodes require an upstream image registry during installation. For an air-gapped installation, use the [`registry` module](/modules/registry/) in `Managed` mode with an image registry inside your network, populated with `d8 mirror push`. The `Local` mode uses an installation bundle and does not support this scenario.
{% endalert %}

## Adding a worker node

Use the kubeconfig from installation to connect to the cluster.

In the `worker` NodeGroup, set [`nodeType: Static`](/modules/node-manager/cr.html#nodegroup-v1-spec-nodetype) and [`systemType: DeckhouseEngine`](/modules/node-manager/cr.html#nodegroup-v1-spec-systemtype).

1. Boot the machine from `metal.iso` and make sure it is waiting for configuration. A `GET` request to `/whoami` returns `installer`.

1. Add the machine to the `worker` group:

   ```shell
   d8 node add 192.168.1.20 --group worker
   ```

   The command reads the group's NodeConfig template and the machine's disks. It determines the system disk, network configuration method, and node name, then sends the configuration to the machine and waits for the node to register in the cluster.

   For automation, pass the settings as command-line flags:

   ```shell
   d8 node add 192.168.1.20 --group worker \
     --name worker-0 \
     --disk-selector serial=S3Z8NB0K700002 \
     --network dhcp \
     --yes
   ```

   The `--dry-run` flag previews the result with secrets redacted and skips sending the configuration.

1. Check that the node has registered in the cluster and its NodeConfig is applied:

   ```shell
   d8 k get nodes
   d8 k get nodeconfigs.internal.deckhouse.io
   ```

The `d8 node add` command requires permission to read `nodeconfigtemplates` and network access to TCP port `50000` on the machine. The [subsystem role `d8:subsystem:cluster:manager`](../admin/configuration/access/authorization/rbac-experimental.html#system-and-subsystem-roles) grants this read permission through the `d8:system-capability:node-manager:edit` capability. Add machines from a trusted network because the installer accepts configuration on this port without authenticating the sender.
