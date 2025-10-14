---
title: "Deckhouse CLI overview and installation"
permalink: en/cli/d8/
description: Deckhouse CLI is a command line interface for cluster management created by the Deckhouse team.
---

Deckhouse CLI is a command line interface for cluster management created by the developers of Deckhouse Kubernetes Platform (DKP). Starting with version 1.59, the D8 CLI is automatically installed on all cluster nodes. You can also [install](#how-do-i-install-deckhouse-cli) the CLI on any machine and use it to operate clusters that are not managed by DKP.

On the command line, the utility can be invoked using the `d8` alias. All the commands are grouped by their function:

{% alert level="info" %}
The `d8 dk` and `d8 mirror` command groups are not available for Community Edition (CE) and Basic Edition (BE).
{% endalert %}

* `d8 k` — the `kubectl` command family.  
    For example, `d8 k get pods` is the same as `kubectl get pods`.
* `d8 dk` — the range of delivery-related commands (see the `werf` tool).  
    For example, you can run `d8 dk plan --repo registry.deckhouse.io` instead of `werf plan --repo registry.deckhouse.io`.

* `d8 mirror` — the range of commands that allow you to copy DKP distribution images to a private container registry (previously the `dhctl mirror` tool was used for this purpose).
  For example, you can run `d8 mirror pull -l <LICENSE> <TAR-BUNDLE-PATH>` instead of `dhctl mirror --license <LICENSE> --images-bundle-path <TAR-BUNDLE-PATH>`.

  The `--only-extra-images` flag allows pulling only extra images for modules (such as vulnerability databases) without downloading the main module images.

  Usage scenarios:

  - [Manually uploading images to an air-gapped registry](../../installing/#manual-loading-of-deckhouse-kubernetes-platform-images-vulnerability-scanner-db-and-dkp-modules-into-a-private-registry).
  - Updating module extra images (such as vulnerability databases): `d8 mirror pull --include-module <module-name> --only-extra-images bundle.tar`.
  - [Updating the platform, modules, and vulnerability databases in air-gapped environment](/products/kubernetes-platform/guides/airgapped-update.html#example-workflow-for-updating-the-platform-modules-and-vulnerability-databases).

* `d8 v` — the set of commands for managing virtual machines created by [Deckhouse Virtualization Platform](/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html).  
    For example, the `d8 virtualization console` command execs you into the VM console.

    <div markdown="0">
    <details><summary>More virtualization commands...</summary>
    <ul>
    <li><code>d8 v console</code> execs you into the VM console.</li>
    <li><code>d8 v port-forward</code> forwards local ports to the virtual machine.</li>
    <li><code>d8 v scp</code> uses the SCP client to work with files on the virtual machine.</li>
    <li><code>d8 v ssh</code> connects you to the virtual machine over SSH.</li>
    <li><code>d8 v vnc</code> connects you to the virtual machine over VNC.</li>
    </ul>
    </details>
    </div>

* `d8 backup` — commands for creating backups of key cluster components:

  * `etcd` — full backup of the etcd key-value store;
  * `cluster-config` — archive of configuration objects;
  * `loki` — export of logs from the built-in Loki API.

    Example:

    ```console
    d8 backup etcd ./etcd.snapshot
    d8 backup cluster-config ./cluster-config.tar
    d8 backup loki --days 1 > ./loki.log
    ```

    You can get the list of available d8 backup flags by running `d8 backup --help`.

## How do I install the Deckhouse CLI?

There are two ways to install the Deckhouse CLI:

* Starting with version 0.10, you can install using [trdl](https://trdl.dev/). This method allows you to get fresh versions of the tool with all improvements and fixes.
  > Note that trdl installation requires Internet access to the tuf repository containing the tool. This method will not work in an air-gapped environment!
* You can manually download the executable file and install it on the system.

### trdl-based installation

Starting with the Deckhouse CLI version 0.10, you can install it using [trdl](https://trdl.dev/).

{% alert level="warning" %}
Versions earlier than 0.10 must be uninstalled before proceeding.

If you need to install one of the versions below 0.10, use the [outdated installation method](#installing-the-executable).
{% endalert %}

1. [Install trdl client](https://trdl.dev/quickstart.html#installing-the-client).

1. Add the Deckhouse CLI repository to trdl:

   ```bash
   URL=https://deckhouse.ru/downloads/deckhouse-cli-trdl
   ROOT_VERSION=1
   ROOT_SHA512=343bd5f0d8811254e5f0b6fe292372a7b7eda08d276ff255229200f84e58a8151ab2729df3515cb11372dc3899c70df172a4e54c8a596a73d67ae790466a0491
   REPO=d8

   trdl add $REPO $URL $ROOT_VERSION $ROOT_SHA512
   ```

1. Install the latest stable release of the `d8` utility and check if it works as expected:

   ```bash
   . $(trdl use d8 0 stable) && d8 --version
   ```

To avoid having to run `. $(trdl use d8 0 stable)` before every Deckhouse CLI invocation, add the following line to your shell’s RC file: `alias d8='trdl exec d8 0 stable -- "$@"'`.

Congratulations, you have installed the Deckhouse CLI.

### Installing the executable

{% include d8-cli-install/main.liquid %}
