---
title: "Deckhouse CLI overview and installation"
permalink: en/cli/d8/
description: Deckhouse CLI is a command line interface for cluster management created by the Deckhouse team.
search: d8, deckhouse cli, d8 utility, command line interface
---

Deckhouse CLI (`d8`) is a command-line interface for working with products in the Deckhouse ecosystem. It combines commands for working with Kubernetes, administering Deckhouse Kubernetes Platform, delivering applications, copying images, creating backups, collecting diagnostic information, virtualization, working with user data, and other tasks.

Starting from release 1.59, `d8` is automatically installed on all DKP cluster nodes. The utility can also be [installed](#how-do-i-install-the-deckhouse-cli) on a separate administrator machine.

## Main command groups

{% alert level="info" %}
The `d8 delivery-kit` and `d8 mirror` command groups are not available in Community Edition (CE) and Basic Edition (BE) editions.
{% endalert %}

`d8` commands are grouped by purpose:

| Commands | Purpose                                                                                                                                              |
|---|------------------------------------------------------------------------------------------------------------------------------------------------------|
| [`d8 k`](../d8/reference/#d8-k) | Commands for managing a cluster (functionally equivalent to `kubectl`).                                                                              |
| [`d8 system`](../d8/reference/#d8-system) | Commands for working with platform system settings, configuration, modules, queues, packages, and system logs.                                       |
| [`d8 status`](../d8/reference/#d8-status) | Command for obtaining the current platform status.                                                                                                   |
| [`d8 backup`](../d8/reference/#d8-backup) | Commands for creating backups of key cluster data and generating diagnostic dumps.                                                                   |
| [`d8 mirror`](../d8/reference/#d8-mirror) | Commands for copying the platform component images to the local filesystem or a third-party container registry.                                      |
| [`d8 delivery-kit`](../d8/reference/#d8-delivery-kit) | Commands for building, publishing, planning, and deploying applications, as well as for working with Helm, registries, SBOM, and related operations. |
| [`d8 data`](../d8/reference/#d8-data) | Commands for managing data, as well as exporting and importing it.                                                                                   |
| [`d8 v`](../d8/reference/#d8-v) | Commands for working with virtual machines created a cluster.                                                                                        |
| [`d8 user`](../d8/reference/#d8-user) | Commands for managing Deckhouse users.                                                                                                               |
| [`d8 stronghold`](../d8/reference/#d8-stronghold) | Commands for managing the lifecycle of secrets (stronghold module)                                                                                                        |
| [`d8 network`](../d8/reference/#d8-network) | Commands for performing network-related operations in the Deckhouse ecosystem, including CNI migration.                                              |
| [`d8 completion`](../d8/reference/#d8-completion) | Commands for generating shell completion scripts.                                                                                                    |
| [`d8 tools`](../d8/reference/#d8-tools) | Auxiliary utilities.                                                                                                                                 |
| [`d8 help`](../d8/reference/#d8-help) | Built-in help.                                                                                                                                       |

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
