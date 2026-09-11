---
title: "Connecting to a virtual machine"
permalink: en/user/virtualization/vm-access.html
description: "Connecting to a virtual machine over the serial console, VNC, and SPICE, plus the run policy and state management."
search: connecting to a VM, console, VNC, SPICE, runPolicy, starting a VM
---

You can connect to a running machine from the command line and from the web interface, while the run policy controls whether the machine is running at all.

## Connecting to a virtual machine

You can connect to a virtual machine in four ways. The first is a remote management protocol such as SSH, which you configure in the guest OS yourself. The second is the serial console. The third is VNC. The fourth is SPICE, if it is enabled for the machine.

{% tabs vm-connect %}

{% tab "Using the CLI" %}

Serial console:

```bash
d8 v console linux-vm
```

Example output:

```console
Successfully connected to linux-vm console. The escape sequence is ^]
linux-vm login: cloud
Password: cloud
```
{: .nowrap-default }

To exit the console, press `Ctrl+]`.

Connecting over VNC:

```bash
d8 v vnc linux-vm
```

Connecting over SPICE:

```bash
d8 v spice linux-vm
```

Connecting over SSH:

```bash
d8 v ssh cloud@linux-vm
```

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Virtual machines**.
1. Select the VM you need from the list and click its name.
1. Go to the **TTY** tab to work with the serial console, or to the **VNC** tab to connect over VNC.

{% endtab %}

{% endtabs %}

{% alert level="warning" %}
The serial console and VNC are exclusive, only one user works in them, and a new connection disconnects whoever is already working with the machine.

Before connecting, `d8 v console` and `d8 v vnc` report who took the stream and since when, and offer a choice:

```console
The serial console of linux-vm is in use:
  user       serviceaccount default/alice
  connected  12 minutes ago (14:32), from the d8 v command line

Connect and disconnect them? [y] yes  [N] no  [w] wait until free:
```
{: .nowrap-default }

The `w` answer means waiting until the other user disconnects and connecting automatically. Pressing Enter cancels the connection, because the safe option is selected by default. The `--force` flag connects without asking, and it's also what you need for a non-interactive run in a script.
{% endalert %}

{% alert level="info" %}
The serial console doesn't resize the terminal automatically. If the command output wraps incorrectly, set the size manually with the `stty rows <ROWS> cols <COLUMNS>` command, for example `stty rows 50 cols 200`. When the `xterm` package is installed in the system, the `resize` command does the same job.
{% endalert %}

### SPICE

SPICE is a second remote display protocol that, unlike VNC, brings the sound of the guest system, redirects USB devices from your computer into it, and shares a clipboard with it. It works alongside VNC, so open VNC sessions and the web interface keep working.

SPICE is disabled by default. To turn it on, set the [`.spec.spice.enabled`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-spice-enabled) parameter and restart the machine:

```yaml
spec:
  spice:
    enabled: true
```

Together with SPICE, the machine gets a virtio-gpu video adapter. If the guest system has no driver for it, as in Windows 7 and Windows XP, set another adapter model with the `virtualization.deckhouse.io/video` annotation, using the `vga`, `bochs`, or `ramfb` value.

A shared clipboard, a resize to the client window, and a local cursor are added by the SPICE guest agent. Install it in the guest system, on Linux it's the `spice-vdagent` package, on Windows it's `spice-guest-tools`.

Connecting requires the `remote-viewer` client from the `virt-viewer` package, and the `d8 v spice` command opens it for you. If you don't have such a client, run the proxy alone and connect with your own client to the port the command prints:

```bash
d8 v spice linux-vm --proxy-only
```

{% alert level="warning" %}
The SPICE display is exclusive the same way the serial console and VNC are, and `d8 v spice` warns you before disconnecting whoever is already connected.

SPICE reserves memory whether a client is connected or not. This memory is part of the machine overhead, so on the node the machine takes more memory than its specification asks for.
{% endalert %}

## Startup policy and VM state management

The startup policy determines how the module maintains the machine state. It's set by the [`.spec.runPolicy`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-runpolicy) parameter:

- `AlwaysOnUnlessStoppedManually`: The default option. The machine always runs, and you can stop it only manually.
- `AlwaysOn`: The machine always runs, and even after a shutdown from the guest OS the module starts it again.
- `Manual`: You manage the machine state yourself.
- `AlwaysOff`: The machine is always off, and you can't start it.

You can manage the machine state in two ways, by creating a [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) resource or by using the `d8` utility. The resource describes the action declaratively, and the utility creates the same resource for you.

| `d8` command   | Operation type | Action                       |
| -------------- | -------------- | ---------------------------- |
| `d8 v stop`    | `Stop`         | Stop the VM                  |
| `d8 v start`   | `Start`        | Start the VM                 |
| `d8 v restart` | `Restart`      | Restart the VM               |
| `d8 v evict`   | `Evict`        | Evict the VM to another node |
| `d8 v migrate` | `Migrate`      | Migrate the VM to another node |

{% tabs vm-operations %}

{% tab "Using the CLI" %}

The easiest way to restart a machine is with the `d8` utility:

```bash
d8 v restart linux-vm
```

The same operation with a resource:

```bash
d8 k create -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineOperation
metadata:
  generateName: restart-linux-vm-
spec:
  virtualMachineName: linux-vm
  # Type of the operation to perform.
  type: Restart
EOF
```

The list of operations shows the result:

```bash
d8 k get virtualmachineoperation

# Short form of the command.
d8 k get vmop
```

{% endtab %}

{% tab "Using the web interface" %}

The startup policy is set on the machine page:

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Virtual machines**.
1. Select the VM you need from the list and click its name.
1. On the **Configuration** tab, scroll down to the **Life cycle** section.
1. Select the policy you need from the **Startup policy** list.

Operations are available from the machine list:

1. Go to **Virtualization** → **Virtual machines**.
1. Select the virtual machine you need from the list and click the ellipsis button.
1. In the menu that opens, select the operation.

{% endtab %}

{% endtabs %}

Only one operation runs at a time for a single machine. A new operation either supersedes the active one or fails, and which of the two happens depends on the pair of types:

| Active operation                        | What can supersede it                     |
| --------------------------------------- | ----------------------------------------- |
| `Start`                                 | `Stop`                                    |
| `Stop` or `Restart` without `force`     | `Stop` or `Restart` with `force: true`    |
| `Migrate`, `Evict`                      | `Stop`, `Restart`                         |
| `Stop` or `Restart` with `force: true`  | nothing                                   |

A superseded operation moves to the `Superseded` phase. An operation that can't supersede the active one moves to the `Failed` phase, so you have to create it again after the active one finishes. Restore and clone operations don't supersede other operations.
