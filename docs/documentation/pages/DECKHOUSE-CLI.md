---
title: Deckhouse CLI
permalink: en/deckhouse-cli/
---

Deckhouse CLI is a command line interface for cluster management created by the developers of Deckhouse Kubernetes Platform (DKP). Starting with version 1.59, the DH CLI is automatically installed on all cluster nodes. You can also [install](#how-do-i-install-deckhouse-cli) the CLI on any machine and use it to operate clusters that are not managed by DKP.

On the command line, the utility can be invoked using the `d8` alias. All the commands are grouped by their function:
* `d8 k` — the `kubectl` command family.  
    For example, `d8 k get pods` is the same as `kubectl get pods`.
* `d8 d` — the range of delivery-related commands (see the `werf` tool).  
    For example, you can run `d8 d plan --repo registry.deckhouse.io` instead of `werf plan --repo registry.deckhouse.io`.

* `d8 mirror` — the range of commands that allow you to copy DKP distribution images to a private container registry (previously the `dhctl mirror` tool was used for this purpose).
  For example, you can run `d8 mirror pull -l <LICENSE> <TAR-BUNDLE-PATH>` instead of `dhctl mirror --license <LICENSE> --images-bundle-path <TAR-BUNDLE-PATH>`.

  > The `d8 d` and `d8 mirror` command groups are not available for Community Edition (CE) and Basic Edition (BE).

* `d8 v` — the set of commands for managing virtual machines created by [Deckhouse Virtualization Platform](/modules/virtualization/stable/).  
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

{% include d8-cli-install/main.liquid %}
