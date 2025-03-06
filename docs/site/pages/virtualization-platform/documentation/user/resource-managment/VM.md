---
title: "Virtual machines"
permalink: en/virtualization-platform/documentation/user/resource-management/virtual-machines.html
---

For creating a virtual machine, the [VirtualMachine](../../../reference/cr/virtualmachine.html) resource is used. Its parameters allow you to configure:

- [Virtual machine class](../../admin/platform-management/virtualization/virtual-machine-classes.html);
- Resources required for the virtual machine (CPU, memory, disks, and images);
- Node placement policies for the virtual machine in the cluster;
- Bootloader settings and optimal parameters for the guest OS;
- Virtual machine startup policy and change application policy;
- Initial configuration scripts (cloud-init);
- List of block devices.

## Creating a virtual machine

Below is an example of a simple virtual machine configuration that runs Ubuntu 22.04. The example uses a cloud-init script that installs the `qemu-guest-agent` and `nginx` services, as well as creates the user `cloud` with the password `cloud`.

The password in this example was generated using the command `mkpasswd --method=SHA-512 --rounds=4096 -S saltsalt`. You can change it to your own if needed.

Create a virtual machine [with a disk](./disks.html#creating-a-disk-from-an-image):

```yaml
d8 k apply -f - <<"EOF"
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: linux-vm
spec:
  # Virtual machine class name.
  virtualMachineClassName: host
  # Cloud-init script block for provisioning the VM.
  provisioning:
    type: UserData
    # Example cloud-init script to create the user "cloud" with the password "cloud" and install the qemu-guest-agent and nginx services.
    userData: |
      #cloud-config
      package_update: true
      packages:
        - nginx
        - qemu-guest-agent
      run_cmd:
        - systemctl daemon-reload
        - systemctl enable --now nginx.service
        - systemctl enable --now qemu-guest-agent.service
      ssh_pwauth: True
      users:
      - name: cloud
        passwd: '$6$rounds=4096$saltsalt$fPmUsbjAuA7mnQNTajQM6ClhesyG0.yyQhvahas02ejfMAq1ykBo1RquzS0R6GgdIDlvS.kbUwDablGZKZcTP/'
        shell: /bin/bash
        sudo: ALL=(ALL) NOPASSWD:ALL
        lock_passwd: False
      final_message: "The system is finally up, after $UPTIME seconds"
  # VM resource settings.
  cpu:
    # Number of CPU cores.
    cores: 1
    # Request 10% of a physical core's CPU time.
    coreFraction: 10%
  memory:
    # Amount of RAM.
    size: 1Gi
  # List of disks and images used in the VM.
  blockDeviceRefs:
    # The order of disks and images in this block determines the boot priority.
    - kind: VirtualDisk
      name: linux-vm-root
EOF
```

After creation, the `VirtualMachine` resource can be in the following states:

- `Pending` - waiting for the readiness of all dependent resources required to start the virtual machine.
- `Starting` - the process of starting the virtual machine is in progress.
- `Running` - the virtual machine is running.
- `Stopping` - the process of stopping the virtual machine is in progress.
- `Stopped` - the virtual machine is stopped.
- `Terminating` - the virtual machine is being deleted.
- `Migrating` - the virtual machine is in the process of online migration to another node.

Check the state of the virtual machine after creation:

```shell
d8 k get vm linux-vm
```

Example output:

```console
NAME       PHASE     NODE           IPADDRESS     AGE
linux-vm   Running   virtlab-pt-2   10.66.10.12   11m
```

After creation, the virtual machine will automatically receive an IP address from the range specified in the module settings (block `virtualMachineCIDRs`).

## Connecting to a virtual machine

There are several ways to connect to a virtual machine:

- Remote management protocol (such as SSH), which must be preconfigured on the virtual machine.
- Serial console.
- VNC protocol.

Example of connecting to a virtual machine using the serial console:

```shell
d8 v console linux-vm
```

Example output:

```console
Successfully connected to linux-vm console. The escape sequence is ^]

linux-vm login: cloud
Password: cloud
```

To exit the serial console, press `Ctrl+]`.

Example command to connect via VNC:

```bash
d8 v vnc linux-vm
```

Example command to connect via SSH:

```bash
d8 v ssh cloud@linux-vm --local-ssh
```

## Startup policy and virtual machine state management

The startup policy of a virtual machine is designed for automated management of the virtual machine's state. It is defined as the `.spec.runPolicy` parameter in the virtual machine's specification. The following policies are supported:

- `AlwaysOnUnlessStoppedManually` — (default) the VM remains running after creation. If a failure occurs, the VM is automatically restarted. Stopping the VM is only possible by calling the `d8 v stop` command or creating the corresponding operation.
- `AlwaysOn` — the VM remains running after creation, even if it is shut down by the OS. If a failure occurs, the VM is automatically restarted.
- `Manual` — after creation, the VM state is managed manually by the user using commands or operations.
- `AlwaysOff` — the VM remains off after creation. Turning it on via commands or operations is not possible.

The state of the virtual machine can be managed using the following methods:

- Creating a [VirtualMachineOperation](../../../reference/cr/virtualmachineoperation.html) (`vmop`) resource.
- Using the [`d8`](../../../reference/console-utilities/d8.html) utility with the corresponding subcommand.

The `VirtualMachineOperation` resource declaratively defines an action that should be performed on the virtual machine.

Example operation to perform a reboot on the virtual machine named `linux-vm`:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineOperation
metadata:
  name: restart-linux-vm-$(date +%s)
spec:
  virtualMachineName: linux-vm
  # Type of operation being applied = Restart operation.
  type: Restart
EOF
```

You can view the result of the action using the following command:

```shell
d8 k get virtualmachineoperation
# or
d8 k get vmop
```

A similar action can be performed using the `d8` utility:

```shell
d8 v restart  linux-vm
```

The possible operations:

| d8             | vmop type | Action                                     |
| -------------- | --------- | ------------------------------------------ |
| `d8 v stop`    | `Stop`    | Stop the VM                                |
| `d8 v start`   | `Start`   | Start the VM                               |
| `d8 v restart` | `Restart` | Restart the VM                             |
| `d8 v evict`   | `Evict`   | Migrate the VM to another, arbitrary node  |

## Changing the configuration of a virtual machine

The configuration of a virtual machine can be modified at any time after the `VirtualMachine` resource is created. However, how these changes are applied depends on the current phase of the virtual machine and the nature of the changes.

You can make changes to the virtual machine's configuration using the following command:

```bash
d8 k edit vm linux-vm
```

If the virtual machine is in a stopped state (`.status.phase: Stopped`), the changes will take effect as soon as it is started.

If the virtual machine is running (`.status.phase: Running`), the method of applying the changes depends on their type:

| Configuration Block                        | How the changes are applied |
| ------------------------------------------ | --------------------------- |
| `.metadata.labels`                         | Applied immediately         |
| `.metadata.annotations`                    | Applied immediately         |
| `.spec.runPolicy`                          | Applied immediately         |
| `.spec.disruptions.restartApprovalMode`    | Applied immediately         |
| `.spec.*`                                  | Requires a VM restart       |

Let's consider an example of changing the virtual machine's configuration:

Suppose we want to change the number of CPU cores. Currently, the virtual machine is running and using one core, which can be confirmed by connecting to it via the serial console and running the `nproc` command.

```shell
d8 v ssh cloud@linux-vm --local-ssh --command "nproc"
```

Example output:

```console
# 1
```

Apply the following patch to the virtual machine to change the number of CPU cores from 1 to 2.

```shell
d8 k patch vm linux-vm --type merge -p '{"spec":{"cpu":{"cores":2}}}'
```

Example output:

```console
virtualmachine.virtualization.deckhouse.io/linux-vm patched
```

The configuration changes have been made, but they have not been applied to the virtual machine yet. Verify this by running the following command again:

```shell
d8 v ssh cloud@linux-vm --local-ssh --command "nproc"
```

Example output:

```console
# 1
```

To apply this change, a restart of the virtual machine is required. Run the following command to see the changes that are pending application (which require a restart):

```shell
d8 k get vm linux-vm -o jsonpath="{.status.restartAwaitingChanges}" | jq .
```

Example output:

```console
[
  {
    "currentValue": 1,
    "desiredValue": 2,
    "operation": "replace",
    "path": "cpu.cores"
  }
]
```

Run the following command:

```shell
d8 k get vm linux-vm -o wide
```

Example output:

```console
NAME        PHASE     CORES   COREFRACTION   MEMORY   NEED RESTART   AGENT   MIGRATABLE   NODE           IPADDRESS     AGE
linux-vm   Running   2       100%           1Gi      True           True    True         virtlab-pt-1   10.66.10.13   5m16s
```

In the `NEED RESTART` column, we see `True`, which indicates that a restart is required to apply the changes.

Let's restart the virtual machine:

```shell
d8 v restart linux-vm
```

After the restart, the changes will be applied, and the `.status.restartAwaitingChanges` block will be empty.

Run the following command to verify:

```shell
d8 v ssh cloud@linux-vm --local-ssh --command "nproc"
# 2
```

By default, the changes to a virtual machine are applied through a manual restart. If you need the changes to be applied immediately and automatically, you can modify the change application policy as follows:

```yaml
spec:
  disruptions:
    restartApprovalMode: Automatic
```

## Initial configuration scripts

Initial configuration scripts are used for the initial setup of a virtual machine when it starts.

The following types of initialization scripts are supported:

- [CloudInit](https://cloudinit.readthedocs.io).
- [Sysprep](https://learn.microsoft.com/en-us/windows-hardware/manufacture/desktop/sysprep--system-preparation--overview).

A CloudInit script can be embedded directly within the VM specification, but this script is limited to a maximum length of 2048 bytes:

```yaml
spec:
  provisioning:
    type: UserData
    userData: |
      #cloud-config
      package_update: true
      ...
```

For longer initialization scripts or when private data is involved, the initialization script for the virtual machine can be created in a Secret resource. Below is an example of a Secret with a CloudInit script:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cloud-init-example
data:
  userData: <base64 data>
type: provisioning.virtualization.deckhouse.io/cloud-init
```

Here is a fragment of the virtual machine configuration when using a CloudInit initialization script stored in a Secret resource:

```yaml
spec:
  provisioning:
    type: UserDataRef
    userDataRef:
      kind: Secret
      name: cloud-init-example
```

Note: The value of the `.data.userData` field must be Base64 encoded.

For configuring virtual machines running Windows using Sysprep, only the Secret option is supported.

Here is an example of a secret with a Sysprep script:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: sysprep-example
data:
  unattend.xml: <base64 data>
type: provisioning.virtualization.deckhouse.io/sysprep
```

Note: The value of the `.data.unattend.xml` field must be Base64 encoded.

Here is the configuration fragment for a virtual machine using the Sysprep initialization script stored in a Secret resource:

```yaml
spec:
  provisioning:
    type: SysprepRef
    sysprepRef:
      kind: Secret
      name: sysprep-example
```

## Placement of virtual machines on nodes

To manage the placement of virtual machines on nodes, you can use the following approaches:

- Simple label binding — `nodeSelector`;
- Preferred binding — `Affinity`;
- Avoid co-location — `AntiAffinity`.

> You can change the placement parameters of virtual machines in real time (available only in the Enterprise edition). However, if the new placement parameters do not match the current ones, the virtual machine will be moved to nodes that meet the new requirements.

### Simple label binding — `nodeSelector`

`nodeSelector` is the simplest way to control the placement of virtual machines using a set of labels. It allows you to specify which nodes can run virtual machines by selecting nodes with the required labels.

```yaml
spec:
  nodeSelector:
    disktype: ssd
```

![nodeSelector](/../../../../images/virtualization-platform/placement-nodeselector.png)

In this example, the virtual machine will be placed only on nodes that have the label `disktype` with the value `ssd`.

### Preferred affinity

`Affinity` provides more flexible and powerful tools compared to `nodeSelector`. It allows defining preferences and requirements for the placement of virtual machines. `Affinity` supports two types: `nodeAffinity` and `virtualMachineAndPodAffinity`.

`nodeAffinity` allows specifying on which nodes the virtual machine can be scheduled using label expressions. It can be either preferred (`preferred`) or mandatory (`required`).

Example of using `nodeAffinity`:

```yaml
spec:
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
          - matchExpressions:
              - key: disktype
                operator: In
                values:
                  - ssd
```

![nodeAffinity](/../../../../images/virtualization-platform/placement-node-affinity.png)

In this example, the virtual machine will be placed only on nodes that have the label `disktype` with the value `ssd`.

`virtualMachineAndPodAffinity` manages the placement of virtual machines relative to other virtual machines. It allows setting preferences to place virtual machines on the same nodes where certain virtual machines are already running.

Example:

```yaml
spec:
  affinity:
    virtualMachineAndPodAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
        - weight: 1
          virtualMachineAndPodAffinityTerm:
            labelSelector:
              matchLabels:
                server: database
            topologyKey: "kubernetes.io/hostname"
```

![virtualMachineAndPodAffinity](/images/virtualization-platform/placement-vm-affinity.png)

In this example, the virtual machine will be placed on nodes that do **not** have any virtual machine labeled with `server: database` on the same node, as the goal is to avoid co-location of certain virtual machines.

### Avoiding Co-Location — AntiAffinity

`AntiAffinity` is the opposite of `Affinity`, and it allows setting requirements to avoid placing virtual machines on the same nodes. This is useful for load distribution or ensuring fault tolerance.

The terms `Affinity` and `AntiAffinity` are applicable only to the relationship between virtual machines. For nodes, the corresponding constraints are referred to as `nodeAffinity`. In `nodeAffinity`, there is no direct opposite term like in `virtualMachineAndPodAffinity`. However, you can create opposing conditions by using negative operators in label expressions. To emphasize excluding certain nodes, you can use `nodeAffinity` with operators like `NotIn`.

Example using `virtualMachineAndPodAntiAffinity`:

```yaml
spec:
  affinity:
    virtualMachineAndPodAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        - labelSelector:
            matchLabels:
              server: database
          topologyKey: "kubernetes.io/hostname"
```

![AntiAffinity](/../../../../images/virtualization-platform/placement-vm-antiaffinity.png)

In this example, the created virtual machine will not be placed on the same node as the virtual machine with the label `server: database`.

## Static and dynamic Block Devices

Block devices can be divided into two types based on how they are connected: static and dynamic (hotplug).

### Static Block Devices

Block devices and their features are presented in the table:

| Block device type | Comment |
| ----------------------- |------------------------------------------------------------------|
| `VirtualImage` | is connected in read-only mode, or as a cd-rom for iso images |
| `ClusterVirtualImage` | is connected in read-only mode, or as a cd-rom for iso images |
| `VirtualDisk` | is connected in read-write mode |

Static block devices are specified in the virtual machine specification in the `.spec.blockDeviceRefs` block as a list. The order of devices in this list determines the sequence in which they are loaded. Thus, if a disk or image is specified first, the bootloader will first try to boot from it. If this fails, the system will move to the next device in the list and try to boot from it. And so on until the first bootloader is detected.

Changing the composition and order of devices in the `.spec.blockDeviceRefs` block is only possible with a reboot of the virtual machine.

A fragment of the VirtualMachine configuration with a statically connected disk and project image:

```yaml
spec:
blockDeviceRefs:
- kind: VirtualDisk
name: <virtual-disk-name>
- kind: VirtualImage
name: <virtual-image-name>
```

### Dynamic Block Devices

Dynamic block devices can be connected and disconnected from a running virtual machine without requiring a reboot.

To attach dynamic block devices, the resource [VirtualMachineBlockDeviceAttachment](../../../reference/cr/virtualmachineblockdeviceattachment.html) (vmbda) is used. Currently, only [VirtualDisk](../../../reference/cr/virtualdisk.html) is supported for attachment as a dynamic block device.

Create the following resource to attach an empty disk `blank-disk` to the virtual machine `linux-vm`:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineBlockDeviceAttachment
metadata:
  name: attach-blank-disk
spec:
  blockDeviceRef:
    kind: VirtualDisk
    name: blank-disk
  virtualMachineName: linux-vm
EOF
```

After creating the `VirtualMachineBlockDeviceAttachment`, it can be in the following states:

- `Pending` - waiting for all dependent resources to be ready.
- `InProgress` - the device attachment process is ongoing.
- `Attached` - the device is successfully attached.

Check the state of your resource:

```shell
d8 k get vmbda attach-blank-disk
```

Example output:

```console
NAME                PHASE      VIRTUAL MACHINE NAME   AGE
attach-blank-disk   Attached   linux-vm              3m7s
```

Connect to the virtual machine and verify that the disk is attached:

```shell
d8 v ssh cloud@linux-vm --local-ssh --command "lsblk"
```

Example output:

```console
NAME    MAJ:MIN RM  SIZE RO TYPE MOUNTPOINTS
sda       8:0    0   10G  0 disk <--- statically attached disk linux-vm-root
|-sda1    8:1    0  9.9G  0 part /
|-sda14   8:14   0    4M  0 part
`-sda15   8:15   0  106M  0 part /boot/efi
sdb       8:16   0    1M  0 disk <--- cloudinit
sdc       8:32   0 95.9M  0 disk <--- dynamically attached disk blank-disk
```

To detach the disk from the virtual machine, delete the previously created resource:

```shell
d8 k delete vmbda attach-blank-disk
```

Attaching images is done in a similar way. To do this, specify VirtualImage or ClusterVirtualImage and the image name as `kind`:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineBlockDeviceAttachment
metadata:
name: attach-ubuntu-iso
spec:
blockDeviceRef:
kind: VirtualImage # Or ClusterVirtualImage.
name: ubuntu-iso
virtualMachineName: linux-vm
EOF
```

## Live migration of virtual machines

Live migration of virtual machines is an important feature in managing virtualized infrastructure. It allows running virtual machines to be moved from one physical node to another without being powered off.

Migration can occur automatically in the following cases:

- Virtual machine firmware updates.
- Load balancing across cluster nodes.
- Putting nodes into maintenance mode for maintenance operations.

Additionally, virtual machine migration can be performed on-demand by the user. Let's look at an example:

Before starting the migration, check the current status of the virtual machine:

```shell
d8 k get vm
```

Example output:

```console
NAME       PHASE     NODE           IPADDRESS     AGE
linux-vm   Running   virtlab-pt-1   10.66.10.14   79m
```

The virtual machine is running on the `virtlab-pt-1` node.

To perform a migration of the virtual machine from one node to another, considering the placement requirements of the virtual machine, use the `VirtualMachineOperation` (`vmop`) resource with the `Evict` type.

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineOperation
metadata:
  name: evict-linux-vm-$(date +%s)
spec:
  # name of the virtual machine
  virtualMachineName: linux-vm
  # operation for migration
  type: Evict
EOF
```

Immediately after creating the `vmop` resource, run the following command:

```shell
d8 k get vm -w
```

Example output:

```console
NAME       PHASE       NODE           IPADDRESS     AGE
linux-vm   Running     virtlab-pt-1   10.66.10.14   79m
linux-vm   Migrating   virtlab-pt-1   10.66.10.14   79m
linux-vm   Migrating   virtlab-pt-1   10.66.10.14   79m
linux-vm   Running     virtlab-pt-2   10.66.10.14   79m
```

You can also perform the migration using the following command:

```shell
d8 v evict <vm-name>
```

## Maintenance mode

When performing work on nodes with running virtual machines, there is a risk of disrupting their functionality. To avoid this, the node can be put into maintenance mode and the virtual machines can be migrated to other free nodes.
To do this, run the following command:

```bash
d8 k drain <nodename> --ignore-daemonsets --delete-emptydir-dat
```

where `<nodename>` is the node on which the work is supposed to be performed and which must be freed from all resources (including system resources).

If you need to evict only virtual machines from a node, run the following command:

```bash
d8 k drain <nodename> --pod-selector vm.kubevirt.internal.virtualization.deckhouse.io/name --delete-emptydir-data
```

After running the `d8 k drain` commands, the node will go into maintenance mode and virtual machines will not be able to start on it. To take it out of maintenance mode, run the following command:

```bash
d8 k uncordon <nodename>
```

![Maintenance mode](/../../../../images/virtualization-platform/drain.png)

## IP Addresses of virtual machines

The `.spec.settings.virtualMachineCIDRs` block in the virtualization module configuration specifies a list of subnets for assigning IP addresses to virtual machines (a shared pool of IP addresses). All addresses in these subnets are available for use, except for the first (network address) and the last (broadcast address).

The `VirtualMachineIPAddressLease` (`vmipl`) resource is a cluster-wide resource that manages the temporary allocation of IP addresses from the shared pool specified in `virtualMachineCIDRs`.

To view the list of temporarily allocated IP addresses (`vmipl`), use the following command:

```shell
d8 k get vmipl
```

Example output:

```console
NAME             VIRTUALMACHINEIPADDRESS                             STATUS   AGE
ip-10-66-10-14   {"name":"linux-vm-7prpx","namespace":"default"}     Bound    12h
```

The [VirtualMachineIPAddress](../../../reference/cr/virtualmachineipaddress.html) (`vmip`) resource is a project or namespace resource responsible for reserving allocated IP addresses and binding them to virtual machines. IP addresses can be assigned automatically or upon request.

To check the assigned IP address, you can use the following command:

```shell
d8 k get vmip
```

Example output:

```console
NAME             ADDRESS       STATUS     VM         AGE
linux-vm-7prpx   10.66.10.14   Attached   linux-vm   12h
```

The algorithm for automatically assigning an IP address to a virtual machine works as follows:

- The user creates a virtual machine with the name `<vmname>`.
- The module controller automatically creates a `vmip` resource with the name `<vmname>-<hash>` to request an IP address and associate it with the virtual machine.
- A `vmipl` lease resource is created for this, which selects a random IP address from the general pool.
- Once the `vmip` resource is created, the virtual machine is assigned the IP address.

By default, the IP address for the virtual machine is automatically assigned from the subnets defined in the module and is bound to it until the virtual machine is deleted. After the virtual machine is deleted, the `vmip` resource is also removed, but the IP address temporarily remains bound to the project/namespace and can be requested again.

## Requesting the required IP address

Create the `vmip` resource:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineIPAddress
metadata:
  name: linux-vm-custom-ip
spec:
  staticIP: 10.66.20.77
  type: Static
EOF
```

Create a new or modify an existing virtual machine and explicitly specify the required `vmip` resource in the specification:

```yaml
spec:
  virtualMachineIPAddressName: linux-vm-custom-ip
```

## Retaining the IP address assigned to a virtual machine

To prevent the automatically assigned IP address of a virtual machine from being deleted along with the virtual machine itself, follow these steps.

Obtain the `vmip` resource name for the specified virtual machine:

```shell
d8 k get vm linux-vm -o jsonpath="{.status.virtualMachineIPAddressName}"
```

Example output:

```console
linux-vm-7prpx
```

Remove the `.metadata.ownerReferences` blocks from the found resource:

```shell
d8 k patch vmip linux-vm-7prpx --type=merge --patch '{"metadata":{"ownerReferences":null}}'
```

After deleting the virtual machine, `the vmip` resource will persist and can be used for a newly created virtual machine:

```yaml
spec:
  virtualMachineIPAddressName: linux-vm-7prpx
```

Even if the `vmip` resource is deleted, it remains leased to the current project/namespace for another 10 minutes, and there is an option to re-lease it upon request:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineIPAddress
metadata:
  name: linux-vm-custom-ip
spec:
  staticIP: 10.66.20.77
  type: Static
EOF
```
