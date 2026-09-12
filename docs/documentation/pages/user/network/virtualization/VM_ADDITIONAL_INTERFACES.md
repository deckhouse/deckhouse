---
title: "Additional network interfaces of virtual machines"
permalink: en/user/network/virtualization/vm-additional-interfaces.html
description: "Additional network interfaces of a virtual machine from sdn module networks and IP address allocation for them."
search: additional interfaces, SDN, ClusterNetwork, IPAddress, project networks
---

Besides the main cluster network, a virtual machine can be connected to additional networks of the [`sdn`](/modules/sdn/) module, and its interfaces can be assigned addresses.

## Additional network interfaces

Besides the main cluster network, a machine can connect to additional networks, both project and cluster ones.

{% alert level="warning" %}
To work with additional networks, the `sdn` module has to be enabled.
{% endalert %}

The following example shows how to connect a virtual machine to an additional network:

{% tabs vm-networks %}

{% tab "Using the CLI" %}

Virtual machines can be connected to additional networks, either project ones (Network) or cluster ones (ClusterNetwork).

To do this, list the networks you need in the [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) block. If this block isn't set (which is the default), the VM uses only the main cluster network.

> You don't have to specify the main cluster network (`type: Main`) in [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks). If you don't need a connection to the main cluster network, you can use only additional networks (`Network` or `ClusterNetwork`).
>
> However, if the main network is specified, it has to be first in the [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) list.

Specifics and important points of working with additional network interfaces:

- the order of networks in [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) determines the order in which interfaces are attached inside the virtual machine;
- adding or removing an additional network (`Network` or `ClusterNetwork`) on a running VM applies without a reboot. The ACPI indexes of existing interfaces are preserved when adding or removing, so interface names in the guest OS stay stable;
- adding or removing the main network (`type: Main`) still requires a VM reboot, because it's bound to the main network interface of the pod and can't be changed on a running pod;
- to preserve the order of network interfaces inside the guest operating system, add new networks to the end of the [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) list and don't change the order of existing ones;
- network security policies (NetworkPolicy) don't apply to additional network interfaces;
- the network parameters (IP addresses, gateways, DNS, and so on) for additional networks are configured manually from inside the guest OS (for example, with Cloud-Init), unless IPAM is configured for the network (for details, see [IPAM for additional network interfaces](#ipam-for-additional-network-interfaces)).

> When configuring network interfaces in the guest OS, use stable identifiers (predictable `enpXsY` names or binding by MAC address) instead of `ethX` names, as described in [Network interface naming in the guest OS](../../virtualization/vm-block-devices.html#network-interface-naming-in-the-guest-os).
>
> On a Linux guest system with several interfaces in the same subnet, the ARP flux problem can occur, where the kernel answers ARP requests through an arbitrary interface rather than the one the request arrived on, which leads to an unstable connection and packet loss because of an incorrect MAC address in the router caches.
>
> To fix this, set the parameters that make the system answer requests strictly through the interface with the target IP and use the correct source address:
>
> ```bash
> sysctl -w net.ipv4.conf.all.arp_ignore=1
> sysctl -w net.ipv4.conf.all.arp_announce=2
> ```
>
> Example for cloud-init:
>
> ```yaml
> write_files:
> - path: /etc/sysctl.d/90-arp-strict.conf
> content: |
> net.ipv4.conf.all.arp_ignore=1
> net.ipv4.conf.all.arp_announce=2
> ```
>
> The parameter values are described in the [IP sysctl documentation](https://docs.kernel.org/networking/ip-sysctl.html).

Here is an example of connecting a VM to the main cluster network and the `user-net` project network:

```yaml
spec:
  networks:
    - type: Main # If specified, it has to be first
    - type: Network # Network type (Network \ ClusterNetwork)
      name: user-net # Network name
```

Here is an example of connecting to several networks, including the `corp-net` cluster network:

```yaml
spec:
  networks:
    - type: Main # If specified, it has to be first
    - type: Network
      name: user-net
    - type: ClusterNetwork
      name: corp-net # Network name
```

Here is an example of connecting a VM only to additional networks (without the main cluster network):

```yaml
spec:
  networks:
    - type: Network
      name: isolated-net
    - type: ClusterNetwork
      name: corp-net
```

You can see the information about the connected networks and their MAC addresses in the VM status:

```yaml
status:
  networks:
    - type: Main
    - type: Network
      name: user-net
      macAddress: aa:bb:cc:dd:ee:01
    - type: ClusterNetwork
      name: corp-net
      macAddress: aa:bb:cc:dd:ee:02
```

For each additional network interface, a unique MAC address is created and reserved automatically, which prevents MAC address collisions. The [VirtualMachineMACAddress](/modules/virtualization/cr.html#virtualmachinemacaddress) (`vmmac`) and [VirtualMachineMACAddressLease](/modules/virtualization/cr.html#virtualmachinemacaddresslease) (`vmmacl`) resources are used for this.

A MAC address is generated at random from a pool of allowed ranges.

- Ranges: `x2-xx-xx-xx-xx-xx`, `x6-xx-xx-xx-xx-xx`, `xA-xx-xx-xx-xx-xx`, `xE-xx-xx-xx-xx-xx`.
- The first three octets (OUI) are formed from the cluster UUID, and the last three (NIC) are picked at random from 16 million possible combinations.

The [VirtualMachineMACAddressLease](/modules/virtualization/cr.html#virtualmachinemacaddresslease) (`vmmacl`) resource is a cluster-wide resource that manages leases of MAC addresses from the shared MAC address pool.

To view the list of MAC address leases (`vmmacl`), run the following command:

```bash
d8 k get vmmacl
```

Example output:

<!-- markdownlint-disable MD031 -->
```console
NAME                    VIRTUALMACHINEMACADDRESS                      STATUS   AGE
mac-5e-e6-19-22-0f-d8   {"name":"vm-01-fz9cr","namespace":"pr-sdn"}   Bound    45s
mac-5e-e6-19-29-89-cf   {"name":"vm-01-99qj6","namespace":"pr-sdn"}   Bound    45s
mac-5e-e6-19-54-f9-be   {"name":"vm-01-5jqxg","namespace":"pr-sdn"}   Bound    45s
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

The [VirtualMachineMACAddress](/modules/virtualization/cr.html#virtualmachinemacaddress) (`vmmac`) resource is a project resource responsible for reserving leased MAC addresses and binding them to virtual machines.

A MAC address is assigned automatically to each additional interface from the shared address pool and stays assigned to the machine until it's deleted.

To check the assigned MAC addresses, run the following command:

```bash
d8 k get vmmac
```

Example output:

<!-- markdownlint-disable MD031 -->
```console
NAME          ADDRESS             STATUS     VM      AGE
vm-01-5jqxg   5e:e6:19:54:f9:be   Attached   vm-01   5m42s
vm-01-99qj6   5e:e6:19:29:89:cf   Attached   vm-01   5m42s
vm-01-fz9cr   5e:e6:19:22:0f:d8   Attached   vm-01   5m42s
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

When a network is removed from the VM configuration:

- The MAC address of the interface is released.
- The related [VirtualMachineMACAddress](/modules/virtualization/cr.html#virtualmachinemacaddress) and [VirtualMachineMACAddressLease](/modules/virtualization/cr.html#virtualmachinemacaddresslease) resources are deleted automatically.
- The allocated `IPAddress` resource is deleted automatically (if IPAM was used).

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Virtual machines**.
1. Select the VM you need from the list and click its name.
1. On the **Configuration** tab, scroll down to the **Networks** section and click **Add**.
1. In the **Add network** window that opens, specify the network you need in the **Select network** field.
1. Click **Add**, then click the **Save** button that appears.

To create a project network:

1. Go to the **Projects** tab and select the project you need.
1. Go to **Network** → **SDN** → **Networks**.
1. Click **Create**.
1. In the **Create resource** window that opens, enter the network name in the **Name** field.
1. On the **Configuration** tab, select the Network class in the **Network class** field, the network type in the **Type** field, and the VLAN ID in the **VLAN** field. Set **Mtu** and the parameters of the **IPAM** block if required.
1. Click **Apply**.
1. The created networks are shown in the list with the **Status**, **Type**, **VLAN**, and **Network class** columns.

{% endtab %}

{% endtabs %}

## IPAM for additional network interfaces

DP can hand out addresses in an additional network itself, if an administrator has configured an address pool for that network.

{% tabs net-ipam %}

{% tab "Using the CLI" %}

If IPAM is configured for an additional network [in the `sdn` module](/modules/sdn/) (an IP address pool bound to the network through [`spec.ipam.ipAddressPoolRef`](/modules/sdn/cr.html#clusternetwork-v1alpha1-spec-ipam-ipaddresspoolref)), DP can automatically allocate IP addresses for the additional VM interfaces and deliver them to the guest OS over DHCP.

Two modes are supported:

- **Automatic (DHCP)**: If the [`ipAddressName` field](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks-ipaddressname) isn't specified in [`.spec.networks[]`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks), the controller automatically creates an IPAddress resource (of the `Auto` type) bound to the VM through `ownerReferences`, and passes it to the `sdn` module. The `sdn` module allocates an address from the pool and delivers it to the guest OS over DHCP. The address is preserved across VM reboots and migrations, because it's bound to the VM rather than to the pod. For this mode to work, the DHCP client has to be enabled on the corresponding interface in the guest OS.

- **Static**: If the [`ipAddressName` field](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks-ipaddressname) is specified in [`.spec.networks[]`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks), the controller uses the IPAddress resource provided by the user (of the `Static` type, `network.deckhouse.io/v1alpha1`). The address is defined by the user and doesn't change automatically.

If an additional network has no IPAM pool configured, the IPAM feature isn't enabled, the interface works in L2-only mode, and IP addressing has to be configured manually in the guest OS.

Here is an example VM configuration with automatic IP address allocation for an additional network:

```yaml
spec:
  networks:
    - type: Main
    - type: ClusterNetwork
      name: corp-net
      # ipAddressName isn't specified → automatic mode (DHCP) is used
```

Here is an example VM configuration with a static IP address for an additional network:

```yaml
spec:
  networks:
    - type: Main
    - type: ClusterNetwork
      name: corp-net
      ipAddressName: my-static-ip # Name of the IPAddress resource (SDN)
```

Here is an example of a static IPAddress resource configuration:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: IPAddress
metadata:
  name: my-static-ip
  namespace: my-namespace
spec:
  networkRef:
    kind: ClusterNetwork
    name: corp-net
  type: Static
  static:
    ip: 192.168.200.42
```

The allocated IP address is shown in the VM status:

```yaml
status:
  ipAddress: 10.66.10.2                     # IP address of the main network.
  virtualMachineIPAddressName: vm-01-main-ip # IPAddress name of the main network.
  networks:
    - type: Main
    - type: ClusterNetwork
      name: corp-net
      macAddress: 32:a6:a1:0a:92:48
      virtualMachineMACAddressName: vm-01-rxzd6
      ipAddress: 192.168.200.4               # IP address of the additional network (from IPAM).
```

> **Important:** If an IPAM pool is configured for an additional network, don't configure a static IP address on the additional interface in the guest OS manually (through Cloud-Init). Use the automatic (DHCP) or static (`ipAddressName`) mode to avoid address conflicts.
>
> If an additional network has an IPAM pool but the IPAddress resource isn't allocated yet or is in the `Pending` state (for example, because the address pool is exhausted), the interface is temporarily skipped, the VM starts without it, and the `NetworkReady` condition reports the error. Once an IP address becomes available, the interface is attached automatically on the fly.

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Network** → **SDN** → **IP pools**.
1. Click **Create**.
1. In the **Create resource** window that opens, enter the pool name in the **Name** field.
1. On the **Configuration** tab, set the lease lifetime in the **Lease TTL** field, and in the **Pools** block, set the network (**Network**), the address ranges (**Ranges**), and the routes (**Routes**).
1. Click **Apply**.

{% endtab %}

{% endtabs %}

### Configuring the guest OS for interfaces added on the fly

When an additional network interface is attached to an already running VM, the guest OS has to be configured to bring new network interfaces up automatically and request a DHCP lease. By default, Linux doesn't start a DHCP client on interfaces added on the fly.

To make such interfaces configure themselves, use one of the following approaches in the guest OS:

- **NetworkManager** (Ubuntu, RHEL, CentOS): Configures new interfaces with DHCP automatically, if the `network-manager` service is running.
- **A udev rule** (Alpine and other systems without `network-manager`): Add a udev rule to bring new interfaces up:

  ```yaml
  write_files:
    - path: /etc/udev/rules.d/90-hotplug-network.rules
      content: |
        SUBSYSTEM=="net", ACTION=="add", RUN+="/sbin/ifup %k"
  ```

Interfaces present at VM boot (included in the initial network configuration) don't need any extra setup, because the guest OS configures them at startup through Cloud-Init.
