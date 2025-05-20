---
title: "Cloud provider — VMware Cloud Director: Preparing environment"
description: "Configuring VMware Cloud Director for Deckhouse cloud provider operation."
---

<!-- AUTHOR! Don't forget to update getting started if necessary -->

## List of required VCD resources

* _Organization_
* _VirtualDataCenter_
* _vApp_
* _StoragePolicy_
* _SizingPolicy_
* _Network_
* _EdgeRouter_
* _Catalog_

Organization, VirtualDataCenter, StoragePolicy, SizingPolicy, EdgeRouter, and Catalog must be provided by your VMware Cloud Director service provider.

Network (internal network) can be configured by your VMware Cloud Director service provider, or you can configure it yourself. Next, we consider setting up the internal network yourself.

### User rights

The user accessing the VMware Cloud Director API must have the following permissions:

* The role of _Organization Administrator_ with the additional permission `Preserve All ExtraConfig Elements During OVF Import and Export`.
* The right `Preserve All ExtraConfig Elements During OVF Import and Export` must be duplicated in the user's _Right Bundle_.

### Adding a network

Go to the _Networking_ tab and click on the _NEW_ button:

![Adding a network, step 1](../../images/cloud-provider-vcd/network-setup/Screenshot.png)

Select the Data Center:

![Adding a network, step 2](../../images/cloud-provider-vcd/network-setup/Screenshot2.png)

Note that _Network type_ must be _Routed_:

![Adding a network, step 3](../../images/cloud-provider-vcd/network-setup/Screenshot3.png)

Connect the _EdgeRouter_ to the network:

![Adding a network, step 4](../../images/cloud-provider-vcd/network-setup/Screenshot4.png)

Set the network name and CIDR:

![Adding a network, step 5](../../images/cloud-provider-vcd/network-setup/Screenshot5.png)

Do not add Static IP Pools, because DHCP will be used:

![Adding a network, step 6](../../images/cloud-provider-vcd/network-setup/Screenshot6.png)

Specify the DNS server addresses:

![Adding a network, step 7](../../images/cloud-provider-vcd/network-setup/Screenshot7.png)

### Configuring DHCP

To provision nodes dynamically, you have to enable the DHCP server for the internal network.

{% alert level="info" %}
We recommend allocating the beginning of the network address range to system consumers (control plane, frontend nodes, system nodes) and the rest to the DHCP pool. For example, for a `/24` mask network it would be enough to allocate 20 addresses to system consumers.
{% endalert %}

Click the _Networking_ tab and open the network you created:

![DHCP, step 1](../../images/cloud-provider-vcd/dhcp-setup/Screenshot.png)

In the window that opens, select the _IP Management_ -> _DHCP_ -> Activate tab:

![DHCP, step 2](../../images/cloud-provider-vcd/dhcp-setup/Screenshot2.png)

In the _General settings_ tab, set the parameters as shown in the example:

![DHCP, step 3](../../images/cloud-provider-vcd/dhcp-setup/Screenshot3.png)

Next, add a pool:

![DHCP, step 3](../../images/cloud-provider-vcd/dhcp-setup/Screenshot4.png)

Set the DNS server addresses:

![DHCP, step 3](../../images/cloud-provider-vcd/dhcp-setup/Screenshot5.png)

### Adding a vApp

Switch to the _Data Centers_ -> _vApps_ -> _NEW_ -> _New vApp_ tab:

![Adding a vApp, step 1](../../images/cloud-provider-vcd/application-setup/Screenshot.png)

Set a name and enable the vApp:

![Adding a vApp, step 2](../../images/cloud-provider-vcd/application-setup/Screenshot2.png)

### Adding a network to the vApp

Once the vApp is created, you have to connect the created internal network to it.

Switch to the _Data Centers_ -> _vApps_ tab and open the target _vApp_:

![Adding a network to the vApp, step 1](../../images/cloud-provider-vcd/network-in-vapp-setup/Screenshot.png)

Go to the _Networks_ tab and click on the _NEW_ button:

![Adding a network to the vApp, step 2](../../images/cloud-provider-vcd/network-in-vapp-setup/Screenshot2.png)

In the window that opens, click the _Direct_ type and select the network:

![Adding a network to the vApp, step 3](../../images/cloud-provider-vcd/network-in-vapp-setup/Screenshot3.png)

### Incoming traffic

Incoming traffic should be routed to the edge router (ports 80, 443) using DNAT rules to be forwarded to a dedicated address on the internal network.  
This address can be created by running MetalLB in L2 mode for dedicated frontend nodes.

### Configuring DNAT/SNAT rules on the edge gateway

Navigate to the _Networking_ -> _Edge Gateways_ tab, open the edge gateway:

![Configuring DNAT rules on the edge gateway, step 1](../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot.png)

Switch to the _Services_ -> _NAT_ tab:

![Configuring DNAT rules on the edge gateway, step 2](../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot2.png)

Add the following rules:

![Configuring DNAT rules on the edge gateway, step 3](../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot3.png)

The first two rules are used for incoming traffic, while the third rule is used for SSH access to the control plane host (without this rule the installation will not be possible).

To enable virtual machines to access the internet, you need to configure SNAT rules. Add the following rules:

![Configuring SNAT rules on the edge gateway, step 1](../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot4.png)

This rule will allow virtual machines from the `192.168.199.0/24` subnet to access the internet.

### Configuring a firewall

Once DNAT is configured, you have to set up the firewall. First, configure the IP sets.

Switch to the _Security_ -> _IP Sets_ tab:

![Configuring the edge gateway firewall, step 1](../../images/cloud-provider-vcd/edge-firewall/Screenshot.png)

Create the following set of IPs (the MetalLB address here is `.10` and the control plane node address is `.2`):

![Configuring the edge gateway firewall, step 1](../../images/cloud-provider-vcd/edge-firewall/Screenshot2.png)

![Configuring the edge gateway firewall, step 1](../../images/cloud-provider-vcd/edge-firewall/Screenshot3.png)

![Configuring the edge gateway firewall, step 1](../../images/cloud-provider-vcd/edge-firewall/Screenshot4.png)

Add the following firewall rules:

![Configuring the edge gateway firewall, step 1](../../images/cloud-provider-vcd/edge-firewall/Screenshot5.png)

## Virtual machine template

{% alert level="warning" %}
The provider is confirmed to work with Ubuntu 22.04-based virtual machine templates only.
{% endalert %}

{% include notice_envinronment.liquid %}

The example below uses the OVA file provided by Ubuntu, updated to include two fixes.
Those fixes are essential for CloudPermanent nodes to be provisioned correctly and to be able to mount CSI-created disks.

### Making a template from an OVA file

Download the [OVA file](https://cloud-images.ubuntu.com/jammy/):

![Setting up the template, step 1](../../images/cloud-provider-vcd/template/Screenshot.png)

Switch to the _Libraries_ -> _Catalogs_ -> _Organization Catalog_ tab:

![Setting up the template, step 2](../../images/cloud-provider-vcd/template/Screenshot2.png)

Select the template you downloaded and add it to the catalog:

![Setting up the template, step 3](../../images/cloud-provider-vcd/template/Screenshot3.png)

![Setting up the template, step 4](../../images/cloud-provider-vcd/template/Screenshot4.png)

![Setting up the template, step 5](../../images/cloud-provider-vcd/template/Screenshot5.png)

Create a virtual machine from the template:

![Setting up the template, step 6](../../images/cloud-provider-vcd/template/Screenshot6.png)

![Setting up the template, step 7](../../images/cloud-provider-vcd/template/Screenshot7.png)

{% alert level="warning" %}
Enter the default password and public key. You will need them to log in to the VM console.
{% endalert %}

![Setting up the template, step 8](../../images/cloud-provider-vcd/template/Screenshot8.png)

Follow these steps to be able to connect to the virtual machine:
1. Start the virtual machine
2. Wait for the IP address to be set
3. _Forward_ port 22 to the virtual machine:

![Setting up the template, step 9](../../images/cloud-provider-vcd/template/Screenshot9.png)

Log on to the virtual machine over SSH and run the following commands:

```shell
echo -e '\n[deployPkg]\nwait-cloudinit-timeout=1800\n' >> /etc/vmware-tools/tools.conf
echo 'disable_vmware_customization: true' > /etc/cloud/cloud.cfg.d/91_vmware_cust.cfg
dpkg-reconfigure cloud-init
```

In the dialog box that appears, leave the checkmark only on `OVF: Reads data from OVF transports`, and make sure to scroll down and remove checkmarks from other options:

![Setting up the template, OVF](../../images/cloud-provider-vcd/template/OVF.png)

Execute the remaining commands:

```shell
truncate -s 0 /etc/machine-id
rm /var/lib/dbus/machine-id
ln -s /etc/machine-id /var/lib/dbus/machine-id
cloud-init clean --logs --seed
passwd -d ubuntu
passwd -d root
rm /home/ubuntu/.ssh/authorized_keys
history -c

shutdown -P now
```

### Setting up the template in VCD

Shut down the virtual machine and clear all populated fields in _Guest Properties_:

![Setting up the template, Guest Properties 1](../../images/cloud-provider-vcd/template/GuestProperties1.png)

![Setting up the template, Guest Properties 5](../../images/cloud-provider-vcd/template/GuestProperties5.png)

Create a virtual machine template:

![Setting up the template, step 10](../../images/cloud-provider-vcd/template/Screenshot10.png)

![Setting up the template, step 11](../../images/cloud-provider-vcd/template/Screenshot11.png)

In the created template, navigate to the _Metadata_ tab and add 6 fields:

* _guestinfo.metadata_
* _guestinfo.metadata.encoding_
* _guestinfo.userdata_
* _guestinfo.userdata.encoding_
* _disk.enableUUID_
* _guestinfo.hostname_

![Setting up the template, Guest Properties 2](../../images/cloud-provider-vcd/template/GuestProperties2.png)

![Setting up the template, Guest Properties 3](../../images/cloud-provider-vcd/template/GuestProperties3.png)

In the vCenter management panel for the template, enable the `disk.EnableUUID` parameter:

![Setting up the template, vCenter 1](../../images/cloud-provider-vcd/template/vCenter1.png)

![Setting up the template, vCenter 2](../../images/cloud-provider-vcd/template/vCenter2.png)

![Setting up the template, vCenter 3](../../images/cloud-provider-vcd/template/vCenter3.png)

![Setting up the template, vCenter 4](../../images/cloud-provider-vcd/template/vCenter4.png)

![Setting up the template, vCenter 5](../../images/cloud-provider-vcd/template/vCenter5.png)

## Using the storage

* VCD supports CSI; disks are created as VCD Independent Disks.
* The `disk.EnableUUID` guest property must be set for the virtual machine templates in use.
* Deckhouse Kubernetes Platform supports disk resizing as of v1.59.1.
