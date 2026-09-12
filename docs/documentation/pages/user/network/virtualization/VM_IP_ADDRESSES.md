---
title: "IP addresses of virtual machines"
permalink: en/user/network/virtualization/vm-ip-addresses.html
description: "IP addresses of virtual machines: requesting a specific address, keeping an address in the project, and shared IPAM for the main network."
search: VM IP address, VirtualMachineIPAddress, IPAM, main network
---

Every virtual machine gets an address in the main cluster network. The sections below cover how to view the assigned address, request a specific one, and keep it in the project.

## IP addresses of VMs

Two resources describe the address of a machine in the main cluster network, the cluster-wide address lease and the address reserved for the project.

{% tabs vmip-list %}

{% tab "Using the CLI" %}

The [`.spec.settings.virtualMachineCIDRs`](../../../admin/configuration/network/vm-network.html) block in the module settings defines the subnets that machines get IP addresses from. All addresses of a subnet are available except the first and the last one.

If an address pool is configured for the main cluster network in the [`sdn`](/modules/sdn/) module, the address of a machine in that network is managed by the shared IPAM of that module, that is, by the same IPAddress resource as for additional networks. The address is requested automatically, and existing machines switch to the shared IPAM without changing their addresses and without a restart.

> **Important:** In a cluster with the shared IPAM, the [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) and [VirtualMachineIPAddressLease](/modules/virtualization/cr.html#virtualmachineipaddresslease) resources, as well as the [`.spec.virtualMachineIPAddressName`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-virtualmachineipaddressname) parameter, are deprecated. They keep working and are described in the sections below, but the main network address of new machines is managed by the shared IPAM, covered in [IPAM for the main network](#ipam-for-the-main-network).

The [VirtualMachineIPAddressLease](/modules/virtualization/cr.html#virtualmachineipaddresslease) (`vmipl`) resource is a cluster-wide resource that manages leases of IP addresses from the shared pool specified in `virtualMachineCIDRs`.

To view the list of IP address leases (`vmipl`), run the following command:

```bash
d8 k get vmipl
```

Example output:

<!-- markdownlint-disable MD031 -->
```console
NAME             VIRTUALMACHINEIPADDRESS                             STATUS   AGE
ip-10-66-10-14   {"name":"linux-vm-7prpx","namespace":"default"}     Bound    12h
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

The [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) (`vmip`) resource is a project resource responsible for reserving leased IP addresses and binding them to virtual machines. IP addresses can be allocated automatically or on explicit request.

An address is assigned to a machine when the resource moves to the `Attached` phase. The other phases are described in the [`.status.phase`](/modules/virtualization/cr.html#virtualmachineipaddress-v1alpha2-status-phase) field.

By default, DP assigns an address to the machine itself and keeps it assigned until the machine is deleted. To view the assigned address, run the following command:

```bash
d8 k get vmip
```

Example output:

<!-- markdownlint-disable MD031 -->
```console
NAME             ADDRESS       STATUS     VM         AGE
linux-vm-7prpx   10.66.10.14   Attached   linux-vm   12h
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

The algorithm for automatically assigning an IP address to a virtual machine looks like this:

- The user creates a virtual machine named `<VM_NAME>`.
- DP automatically creates a [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) resource named `<VM_NAME>-<HASH>` to request an IP address and bind it to the virtual machine.
- For this [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress), a [VirtualMachineIPAddressLease](/modules/virtualization/cr.html#virtualmachineipaddresslease) lease resource is created, which picks a random IP address from the shared pool.
- As soon as the [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) resource is created, the virtual machine gets the assigned IP address.

After the machine is deleted, the [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) resource is deleted too, but the address itself stays assigned to the project for a while, and you can request it again.

All parameters of these resources are described in [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) and [VirtualMachineIPAddressLease](/modules/virtualization/cr.html#virtualmachineipaddresslease).

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **IP addresses**.
1. The list shows the resource name, status, address, type (`Auto` or `Static`), the virtual machine that uses the address, and the resource age.

{% endtab %}

{% endtabs %}

### Assigning a specific IP address

Instead of a random address from the pool, you can give a machine an address you choose in advance.

{% tabs vmip-static %}

{% tab "Using the CLI" %}

1. Create a [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) resource:

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

1. Create a new virtual machine or modify an existing one, and specify the [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) resource you need explicitly in the specification:

   ```yaml
   spec:
     virtualMachineIPAddressName: linux-vm-custom-ip
   ```

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **IP addresses**.
1. Click **Create**.
1. In the **Create resource** window that opens, enter the resource name in the **Name** field.
1. On the **Configuration** tab, select `Static` in the **Type** field, and specify the address you need in the **Static IP address** field.
1. Click **Apply**.
1. Specify the name of the created resource in the [`.spec.virtualMachineIPAddressName`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-virtualmachineipaddressname) parameter of the virtual machine.

{% endtab %}

{% endtabs %}

### Keeping an IP address in the project

To keep the automatically allocated IP address of a virtual machine from being deleted along with the virtual machine itself, do the following.

Get the name of the [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) resource for the given virtual machine:

```bash
d8 k get vm linux-vm -o jsonpath="{.status.virtualMachineIPAddressName}"
```

Example output:

```console
linux-vm-7prpx
```

Remove the `.metadata.ownerReferences` block from the resource you found:

```bash
d8 k patch vmip linux-vm-7prpx --type=merge --patch '{"metadata":{"ownerReferences":null}}'

# Or make the same changes by editing the resource.

d8 k edit vmip linux-vm-7prpx
```

After the virtual machine is deleted, the [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) resource is preserved and you can reuse it in a newly created virtual machine:

```yaml
spec:
  virtualMachineIPAddressName: linux-vm-7prpx
```

Even if the [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) resource is deleted, the IP address stays leased to the current project or namespace for another 10 minutes. So you can claim it again on request:

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

## IPAM for the main network

{% alert level="warning" %}
The shared IPAM works only if an address pool is configured for the main cluster network in the [`sdn`](/modules/sdn/) module. Without it, machine addresses are managed by the deprecated [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) resources described above.
{% endalert %}

The address of a machine in the main network is held by the [IPAddress](/modules/sdn/cr.html#ipaddress) resource of the `sdn` module, the same one that addresses additional networks. There are two ways to get an address:

- **Automatic**: If the [`ipAddressName` field](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks-ipaddressname) isn't set for the `Main` network, the controller creates an IPAddress resource bound to the machine and waits for the address before starting it. A machine that failed to get an address stays in the `Pending` phase, and the reason is shown by the `VirtualMachineIPAddressReady` condition.

- **Static**: Create an IPAddress resource of the main network yourself and specify it in the [`.spec.networks[]`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) parameter. Such a resource belongs to you, so it isn't deleted along with the machine and can be reassigned to another one.

An example of a static IPAddress resource for the main network:

```yaml
d8 k apply -f - <<EOF
apiVersion: network.deckhouse.io/v1alpha1
kind: IPAddress
metadata:
  name: linux-vm-main-ip
spec:
  networkRef:
    kind: ClusterNetwork
    name: main
  type: Static
  static:
    ip: 10.66.20.77
EOF
```

An example of a reference to this resource in the machine specification:

```yaml
spec:
  networks:
    - type: Main
      ipAddressName: linux-vm-main-ip
```

Changing the `ipAddressName` field of the main network changes the address of the primary interface, so it requires a restart of the machine.

The current address and the name of the IPAddress resource it is assigned to are shown in the machine status:

```bash
d8 k get vm linux-vm -o jsonpath='{.status.networks[?(@.type=="Main")]}'
```

Example output:

<!-- markdownlint-disable MD031 -->
```txt
{"id":1,"ipAddress":"10.66.10.14","ipAddressName":"linux-vm-4bkqr","type":"Main"}
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

### Switching a machine to the shared IPAM

Machines that don't reference a [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) resource explicitly switch to the shared IPAM automatically, and the address doesn't change and the machine isn't restarted.

A machine with the [`.spec.virtualMachineIPAddressName`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-virtualmachineipaddressname) parameter set keeps working through the deprecated mechanism, because the reference to the resource is set in its specification explicitly.

For the [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) resource listed in the machine specification, DP maintains an IPAddress resource with the same name and the same address. To switch the machine and keep its address, replace one parameter with the other:

```yaml
spec:
  # Remove the virtualMachineIPAddressName parameter.
  networks:
    - type: Main
      # An IPAddress resource with the same name and address.
      ipAddressName: linux-vm-custom-ip
```

Restart the machine to apply the change. The address stays the same, and the [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) resource can be deleted after that.

{% alert level="warning" %}
Don't remove the [`.spec.virtualMachineIPAddressName`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-virtualmachineipaddressname) parameter without such a replacement, otherwise the machine gets a new address from the pool.
{% endalert %}
