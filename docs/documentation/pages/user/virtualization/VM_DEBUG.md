---
title: "Collecting debug information about a virtual machine"
permalink: en/user/virtualization/vm-debug.html
description: "Collecting debug information about a virtual machine for a support request."
search: debug information, VM diagnostics, support
---

If a machine doesn't behave as expected, collect its state and the state of the related resources into a single archive.

{% alert level="warning" %}
The `collect-debug-info` command requires `d8` v0.27.0 or later.
{% endalert %}

{% tabs vm-debug %}

{% tab "Using the CLI" %}

The `collect-debug-info` command collects diagnostic data about a VM and all related resources into a single compressed archive.

The command collects the following information:

- the virtual machine configuration;
- operations on the virtual machine;
- migration information;
- block devices;
- related PVCs and PVs;
- pods related to the VM, including their logs (the last 10000 lines);
- events for all related resources;
- the XML configuration of the VM domain.

The command result is written to a compressed archive (tar.gz) that goes to stdout. To save the archive, redirect the output to a file.

Usage example:

```bash
# Collect debug information for the 'linux-vm' virtual machine
d8 v collect-debug-info linux-vm > debug-info.tar.gz

# Collect debug information for a VM with the namespace specified
d8 v collect-debug-info linux-vm -n mynamespace > debug-info.tar.gz

# Collect debug information for a VM with the full name specified (name.namespace)
d8 v collect-debug-info linux-vm.mynamespace > debug-info.tar.gz
```

> **Important:** The command can't print data directly to the terminal. Be sure to redirect the output to a file, otherwise the command fails.

After the command runs, you get the `debug-info.tar.gz` archive, which contains all the collected data in YAML format (for resources) and text files (for logs). You can send this archive to technical support to analyze problems.

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Virtual machines**.
1. Select the VM you need from the list and click its name.
1. Go to the **Diagnostics** tab.
1. The **VM pods** block shows the pods of the virtual machine, their phase, and the node they're placed on.
1. In the **VM pod logs** block, you can view the pod logs, filter the lines with a regular expression, and set the number of last lines.
1. To download the archive with diagnostic data, click **Download diagnostic data**.

Current and completed operations on the VM are shown on the **Operations** tab: for each operation, the date, the [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) resource name, the type (**Start**, **Stop**, and others), the status, the progress, and the message are shown; you can limit the list to the **Day**, **Week**, or **Month** period. Events are shown on the **Events** tab, and resource consumption charts on the **Monitoring** tab.

{% endtab %}

{% endtabs %}
