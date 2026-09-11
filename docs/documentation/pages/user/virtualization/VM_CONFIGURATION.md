---
title: "Changing the virtual machine configuration"
permalink: en/user/virtualization/vm-configuration.html
description: "Changing the configuration of a running virtual machine: which parameters apply immediately, which require a restart, and changing cores and memory without a restart."
search: changing VM configuration, VM restart, CPU hotplug, memory hotplug
---

You can change the machine configuration at any time after creation. On a powered-off machine, the changes apply right away, and on a running one it depends on what exactly you changed.

| Configuration block                                                                                             | How it applies on a running VM                                                                                                                                                               |
|-----------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `.metadata.labels`                                                                                              | Right away, and it propagates to the VM pod                                                                                                                                                  |
| `.metadata.annotations`                                                                                         | Right away, and it propagates to the VM pod                                                                                                                                                  |
| [`.spec.liveMigrationPolicy`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-livemigrationpolicy)                         | Right away                                                                                                                                                                                   |
| [`.spec.runPolicy`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-runpolicy)                                             | Right away                                                                                                                                                                                   |
| [`.spec.disruptions.restartApprovalMode`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-disruptions-restartapprovalmode) | Right away                                                                                                                                                                                   |
| [`.spec.affinity`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-affinity)                                               | Right away in commercial DP editions, a restart is required in DP Open                                                                                                                       |
| [`.spec.nodeSelector`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-nodeselector)                                       | Right away in commercial DP editions, a restart is required in DP Open                                                                                                                       |
| [`.spec.cpu.cores`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-cpu-cores)                                             | Without a restart if [changing the number of cores without a restart](#changing-the-number-of-cores-without-a-restart) is enabled in commercial DP editions, otherwise a restart is required |
| [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks)                                               | Adding and removing networks applies on a running VM if the guest OS supports attaching interfaces on the fly                                                                                |
| Other `.spec` fields                                                                                            | A restart is required                                                                                                                                                                        |

The following example shows how to change the configuration of a virtual machine:

{% tabs vm-config %}

{% tab "Using the CLI" %}

The following example changes the number of cores.

1. Check how many cores the guest OS sees now:

   ```bash
   d8 v ssh cloud@linux-vm --command "nproc"
   ```

   Example output:

   ```console
   1
   ```

1. Set the new number of cores:

   ```bash
   d8 k patch vm linux-vm --type merge -p '{"spec":{"cpu":{"cores":2}}}'

   # You can achieve the same result by editing the resource.
   d8 k edit vm linux-vm
   ```

1. Verify that the change is accepted but not applied yet. The guest OS still sees one core, and the list of pending changes isn't empty:

   ```bash
   d8 k get vm linux-vm -o jsonpath="{.status.restartAwaitingChanges}" | jq .
   ```

   Example output:

   ```json
   [
     {
       "currentValue": 1,
       "desiredValue": 2,
       "operation": "replace",
       "path": "cpu.cores"
     }
   ]
   ```

   The `NEED RESTART` column shows the same:

   ```bash
   d8 k get vm linux-vm -o wide
   ```

   Example output:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME       PHASE     UPTIME   CORES   COREFRACTION   MEMORY   NEED RESTART   AGENT   MIGRATABLE   NODE           IPADDRESS     AGE
   linux-vm   Running   5m16s    2       100%           1Gi      True           True    True         virtlab-pt-1   10.66.10.13   5m16s
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. Restart the machine:

   ```bash
   d8 v restart linux-vm
   ```

1. Check the result. After the restart, the [`.status.restartAwaitingChanges`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-status-restartawaitingchanges) block is empty and the guest OS sees two cores:

   ```bash
   d8 v ssh cloud@linux-vm --command "nproc"
   ```

   Example output:

   ```console
   2
   ```

By default, you confirm the restart. To make the module apply the changes itself, set the [`.spec.disruptions.restartApprovalMode`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-disruptions-restartapprovalmode) parameter to `Automatic`:

```yaml
spec:
  disruptions:
    restartApprovalMode: Automatic
```

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Virtual machines**.
1. Select the VM you need from the list and click its name.
1. Make the changes on the **Configuration** tab. If the machine has to be restarted, the module shows a warning and the list of pending changes.
1. To make the changes apply without your confirmation, scroll down to the **Life cycle** section, enable the **Auto-apply changes** toggle, and click **Save**.

{% endtab %}

{% endtabs %}

## Changing the number of cores without a restart

You can change the number of cores of a running machine without rebooting it, if the change is applicable through live migration. Within the current CPU topology, you can both add and remove cores.

The feature is disabled by default. To enable it, an administrator adds `HotplugCPUWithLiveMigration` to the [`.spec.settings.featureGates`](../../admin/configuration/virtualization/settings.html) parameter of the module:

```yaml
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  settings:
    featureGates:
      - HotplugCPUWithLiveMigration
```

In the web interface, the same toggle is called **Change CPU without reboot** and is located in the **Experimental features** block on the **System** tab, in **Deckhouse** → **Modules** → `virtualization` → **Configuration**. Only a platform administrator has the rights for this.

When the feature is enabled and the new [`.spec.cpu.cores`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-cpu-cores) value stays within the current topology, the module applies the change by live migration. If the change requires a different topology, the machine has to be rebooted. The topology calculation rules are described in [CPU topologies](vm-resources.html#cpu-topologies).

{% tabs vm-cpu-change %}

{% tab "Using the CLI" %}

Set the new number of cores:

```bash
d8 k patch vm linux-vm --type merge -p '{"spec":{"cpu":{"cores":4}}}'
```

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Virtual machines**.
1. Select the VM you need from the list and click its name.
1. On the **Configuration** tab, in the **Resources** section, set the new value in the **CPU cores** field.
1. Click the **Save** button that appears.

{% endtab %}

{% endtabs %}

The guest OS doesn't always bring new cores into service on its own, especially after a live migration. In Linux, a core is brought online through sysfs:

```bash
echo 1 > /sys/devices/system/cpu/cpu1/online
```

To make this happen automatically, add a `udev` rule:

<!-- markdownlint-disable MD031 -->
```bash
cat <<'EOF' > /etc/udev/rules.d/99-hotplug-cpu.rules
SUBSYSTEM=="cpu",ACTION=="add",RUN+="/bin/sh -c '[ ! -e /sys$devpath/online ] || echo 1 > /sys$devpath/online'"
EOF
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

The cores brought into service appear in the output of `nproc`, `cat /proc/cpuinfo`, and `top`.

When you reduce the number of cores within the current topology, the distribution of cores across sockets is preserved.

## Changing the amount of memory without a restart

You can increase the amount of memory of a running machine without rebooting it. Reducing it requires a restart.

The feature is disabled by default. To enable it, an administrator adds `HotplugMemoryWithLiveMigration` to the [`.spec.settings.featureGates`](../../admin/configuration/virtualization/settings.html) parameter of the module:

```yaml
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  settings:
    featureGates:
      - HotplugMemoryWithLiveMigration
```

In the web interface, the toggle is called **Change memory without reboot** and is located in the same place, in the **Experimental features** block of the module settings.

When the feature is enabled, the new [`.spec.memory.size`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-memory-size) value is greater than the current one, and the machine allows migration, the module applies the change by live migration. A restart is required if the memory is reduced, if the original size is less than 1 GiB, or if the machine can't be migrated. Without a restart, memory grows up to 256 GiB, the ceiling built into the machine configuration at first start.

{% tabs vm-memory-change %}

{% tab "Using the CLI" %}

Set the new memory size:

```bash
d8 k patch vm linux-vm --type merge -p '{"spec":{"memory":{"size":"4Gi"}}}'
```

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Virtual machines**.
1. Select the VM you need from the list and click its name.
1. On the **Configuration** tab, in the **Resources** section, set the new value in the **Memory size** field.
1. Click the **Save** button that appears.

{% endtab %}

{% endtabs %}

As with cores, the guest OS may not bring new memory blocks into service on its own. In Linux, a block is brought online through sysfs, and the device name is visible in the `lsmem` output or in the `/sys/bus/memory/devices/` directory:

```bash
echo 1 > /sys/bus/memory/devices/memoryXXX/online
```

To make this happen automatically, add a `udev` rule:

<!-- markdownlint-disable MD031 -->
```bash
cat <<'EOF' > /etc/udev/rules.d/99-hotplug-memory.rules
SUBSYSTEM=="memory",ACTION=="add",DEVPATH=="/devices/system/memory/memory[0-9]*", TEST=="state", ATTR{state}!="online", ATTR{state}="online"
EOF
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->
