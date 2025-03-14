---
title: Deckhouse CLI
permalink: en/deckhouse-cli/
description: Deckhouse CLI is a command line interface for cluster management created by the Deckhouse team.
---

Deckhouse CLI is a command line interface for cluster management created by the developers of Deckhouse Kubernetes Platform (DKP). Starting with version 1.59, the D8 CLI is automatically installed on all cluster nodes. You can also [install](#how-do-i-install-deckhouse-cli) the CLI on any machine and use it to operate clusters that are not managed by DKP.

On the command line, the utility can be invoked using the `d8` alias. All the commands are grouped by their function:

{% alert level="info" %}
The `d8 d` and `d8 mirror` command groups are not available for Community Edition (CE) and Basic Edition (BE).
{% endalert %}

* `d8 k` — the `kubectl` command family.  
    For example, `d8 k get pods` is the same as `kubectl get pods`.
* `d8 dk` — the range of delivery-related commands (see the `werf` tool).  
    For example, you can run `d8 d plan --repo registry.deckhouse.io` instead of `werf plan --repo registry.deckhouse.io`.

* `d8 mirror` — the range of commands that allow you to copy DKP distribution images to a private container registry (previously the `dhctl mirror` tool was used for this purpose).
  For example, you can run `d8 mirror pull -l <LICENSE> <TAR-BUNDLE-PATH>` instead of `dhctl mirror --license <LICENSE> --images-bundle-path <TAR-BUNDLE-PATH>`.

  Usage scenarios:

  - [Manually uploading images to an air-gapped registry](/products/kubernetes-platform/documentation/v1/deckhouse-faq.html#manually-uploading-images-to-an-air-gapped-registry).
  - [Manually uploading images of Deckhouse modules into an air-gapped registry](/products/kubernetes-platform/documentation/v1/deckhouse-faq.html#manually-uploading-images-of-deckhouse-modules-into-an-air-gapped-registry).

* `d8 v` — the set of commands for managing virtual machines created by [Deckhouse Virtualization Platform](https://deckhouse.io/products/virtualization-platform/documentation/).  
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

## How do I install Deckhouse CLI?

Starting from Deckhouse CLI 0.10 it can be installed using [trdl](https://trdl.dev/).

{% alert %}
Please note that since version 0.10, installation **is available only via trdl**. If you have a version lower than 0.10 installed, then you must first uninstall it.

If you need to install one of the versions below 0.10, use the [outdated installation method](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.67/deckhouse-cli/#how-do-i-install-deckhouse-cli).
{% endalert %}

1. [Install trdl client](https://trdl.dev/quickstart.html#installing-the-client).

1. Add the Deckhouse CLI repository to trdl:

   ```bash
   URL=https://trrr.flant.dev/trdl-deckhouse-cli
   ROOT_VERSION=0
   ROOT_SHA512=$(curl -Ls ${URL}/root.json | sha512sum | tr -d '\-[:space:]\n')
   REPO=trdl-d8
   
   trdl add $REPO $URL $ROOT_VERSION $ROOT_SHA512
   ```

1. Install stable release:

   ```bash
   trdl update $REPO $ROOT_VERSION stable
   ```

1. Validate that the `d8` binary is installed:

   ```bash
   . $(trdl use $REPO $ROOT_VERSION stable) && d8 --version
   ```

{% alert level="warning" %}
On macOS you might need to remove the quarantine attribute from binary to prevent Gatekeeper from blocking it.
(`sudo xattr -d com.apple.quarantine /path/to/d8`)
{% endalert %}
