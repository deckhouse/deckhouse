---
title: "Cloud provider — VMware Cloud Director: Preparing environment"
description: "Configuring VMware Cloud Director for Deckhouse cloud provider operation."
---

<!-- AUTHOR! Don't forget to update getting started if necessary -->

## List of required VCD resources

* Organization
* VirtualDataCenter
* vApp
* StoragePolicy
* SizingPolicy
* Network
* EdgeRouter
* Catalog

The Organization, VirtualDataCenter, StoragePolicy, SizingPolicy, EdgeRouter, and Catalog resources must be provided by your VMware Cloud Director service provider.

Network (internal network) can be configured by your VMware Cloud Director service provider, or you can configure it yourself. The following sections describe how you can configure the internal network.

### User permissions

The user accessing the VMware Cloud Director API must have the following permissions:

* The role of `Organization Administrator` with the additional permission `Preserve All ExtraConfig Elements During OVF Import and Export`.
* The permission `Preserve All ExtraConfig Elements During OVF Import and Export` must be duplicated in the user's `Right Bundle`.

### Adding a network

1. Go to the **Networking** tab and click **NEW**:

   ![Adding a network, step 1](../../images/cloud-provider-vcd/network-setup/Screenshot.png)

1. Select the Data Center:

   ![Adding a network, step 2](../../images/cloud-provider-vcd/network-setup/Screenshot2.png)

1. At the **Network type** step, select **Routed**:

   ![Adding a network, step 3](../../images/cloud-provider-vcd/network-setup/Screenshot3.png)

1. Connect `EdgeRouter` to the network:

   ![Adding a network, step 4](../../images/cloud-provider-vcd/network-setup/Screenshot4.png)

1. Specify the network name and CIDR:

   ![Adding a network, step 5](../../images/cloud-provider-vcd/network-setup/Screenshot5.png)

1. Do not add **Static IP Pools** because DHCP will be used:

   ![Adding a network, step 6](../../images/cloud-provider-vcd/network-setup/Screenshot6.png)

1. Specify the DNS server addresses:

   ![Adding a network, step 7](../../images/cloud-provider-vcd/network-setup/Screenshot7.png)

### Configuring DHCP

To provision nodes dynamically, enable the DHCP server for the internal network.

{% alert level="info" %}
We recommend allocating the beginning of the network address range to system consumers (control plane, frontend nodes, system nodes) and the rest to the DHCP pool. For example, for a `/24` mask network it would be enough to allocate 20 addresses to system consumers.
{% endalert %}

1. Click the **Networking** tab and open the network you created:

   ![DHCP, step 1](../../images/cloud-provider-vcd/dhcp-setup/Screenshot.png)

1. In the opened window, select **IP Management** -> **DHCP** -> **Activate**:

   ![DHCP, step 2](../../images/cloud-provider-vcd/dhcp-setup/Screenshot2.png)

1. In the **General settings** tab, set the parameters as shown in the example:

   ![DHCP, step 3](../../images/cloud-provider-vcd/dhcp-setup/Screenshot3.png)

1. Add a pool:

   ![DHCP, step 3](../../images/cloud-provider-vcd/dhcp-setup/Screenshot4.png)

1. Set the DNS server addresses:

   ![DHCP, step 3](../../images/cloud-provider-vcd/dhcp-setup/Screenshot5.png)

### Adding a vApp

1. Switch to the **Data Centers** tab -> **vApps** -> **NEW** -> **New vApp**:

   ![Adding a vApp, step 1](../../images/cloud-provider-vcd/application-setup/Screenshot.png)

1. Specify a name and enable the vApp:

   ![Adding a vApp, step 2](../../images/cloud-provider-vcd/application-setup/Screenshot2.png)

### Adding a network to the vApp

Once the vApp is created, connect the created internal network to it.

1. Switch to the **Data Centers** tab -> **vApps** and open the target vApp:

   ![Adding a network to the vApp, step 1](../../images/cloud-provider-vcd/network-in-vapp-setup/Screenshot.png)

1. Go to the **Networks** tab and click **NEW**:

   ![Adding a network to the vApp, step 2](../../images/cloud-provider-vcd/network-in-vapp-setup/Screenshot2.png)

1. In the opened window, click the **Direct** type and select the network:

   ![Adding a network to the vApp, step 3](../../images/cloud-provider-vcd/network-in-vapp-setup/Screenshot3.png)

### Incoming traffic

Incoming traffic should be routed to the edge router (ports `80`, `443`) using DNAT rules to be forwarded to a dedicated address on the internal network.  
This address can be created by running MetalLB in L2 mode for dedicated frontend nodes.

### Configuring DNAT/SNAT rules on the edge gateway

1. Navigate to the **Networking** tab -> **Edge Gateways** and open the edge gateway:

   ![Configuring DNAT rules on the edge gateway, step 1](../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot.png)

1. Switch to the **Services** tab -> **NAT**:

   ![Configuring DNAT rules on the edge gateway, step 2](../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot2.png)

1. Add the following rules:

   ![Configuring DNAT rules on the edge gateway, step 3](../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot3.png)

   The first two rules are used for incoming traffic, while the third rule is used for SSH access to the control plane host (without this rule the installation will not be possible).

1. To allow virtual machines to access the internet, configure SNAT rules following the example:

   ![Configuring SNAT rules on the edge gateway, step 1](../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot4.png)

   This rule will allow virtual machines from the `192.168.199.0/24` subnet to access the internet.

### Configuring a firewall

Once DNAT is configured, set up the firewall. Start by configuring the IP sets.

1. Switch to the **Security** tab -> **IP Sets**:

   ![Configuring the edge gateway firewall, step 1](../../images/cloud-provider-vcd/edge-firewall/Screenshot.png)

1. Create the following set of IPs (the MetalLB address here is `.10` and the control plane node address is `.2`):

   ![Configuring the edge gateway firewall, step 1](../../images/cloud-provider-vcd/edge-firewall/Screenshot2.png)

   ![Configuring the edge gateway firewall, step 1](../../images/cloud-provider-vcd/edge-firewall/Screenshot3.png)

   ![Configuring the edge gateway firewall, step 1](../../images/cloud-provider-vcd/edge-firewall/Screenshot4.png)

1. Add the following firewall rules:

   ![Configuring the edge gateway firewall, step 1](../../images/cloud-provider-vcd/edge-firewall/Screenshot5.png)

## Virtual machine template

{% alert level="warning" %}
The provider is confirmed to work with Ubuntu 22.04-based virtual machine templates only.
{% endalert %}

{% include notice_envinronment.liquid %}

The example below uses the OVA file provided by Ubuntu, updated to include two fixes.
Those fixes are essential for CloudPermanent nodes to be provisioned correctly and to be able to mount CSI-created disks.

### Making a template from an OVA file

1. Download the [OVA file](https://cloud-images.ubuntu.com/jammy/):

   ![Setting up the template, step 1](../../images/cloud-provider-vcd/template/Screenshot.png)

1. Switch to the **Libraries** tab -> **Catalogs** -> **Organization Catalog**:

   ![Setting up the template, step 2](../../images/cloud-provider-vcd/template/Screenshot2.png)

1. Select the template you downloaded and add it to the catalog:

   ![Setting up the template, step 3](../../images/cloud-provider-vcd/template/Screenshot3.png)

   ![Setting up the template, step 4](../../images/cloud-provider-vcd/template/Screenshot4.png)

   ![Setting up the template, step 5](../../images/cloud-provider-vcd/template/Screenshot5.png)

1. Create a virtual machine from the template:

   ![Setting up the template, step 6](../../images/cloud-provider-vcd/template/Screenshot6.png)

   ![Setting up the template, step 7](../../images/cloud-provider-vcd/template/Screenshot7.png)

{% alert level="warning" %}
Enter the default password and public key. You will need them to log in to the VM console.
{% endalert %}

![Setting up the template, step 8](../../images/cloud-provider-vcd/template/Screenshot8.png)

Follow these steps to be able to connect to the virtual machine:

1. Start the virtual machine.
2. Wait for the IP address to be set.
3. _Forward_ port `22` to the virtual machine:

   ![Setting up the template, step 9](../../images/cloud-provider-vcd/template/Screenshot9.png)

Log on to the virtual machine over SSH and run the following commands:

```shell
rm /etc/netplan/99-netcfg-vmware.yaml
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

1. Shut down the virtual machine and clear all populated fields in **Guest Properties**:

   ![Setting up the template, Guest Properties 1](../../images/cloud-provider-vcd/template/GuestProperties1.png)

   ![Setting up the template, Guest Properties 5](../../images/cloud-provider-vcd/template/GuestProperties5.png)

1. Create a virtual machine template:

   ![Setting up the template, step 10](../../images/cloud-provider-vcd/template/Screenshot10.png)

   ![Setting up the template, step 11](../../images/cloud-provider-vcd/template/Screenshot11.png)

1. In the created template, navigate to the **Metadata** tab and add the following six fields:

   * `guestinfo.metadata`
   * `guestinfo.metadata.encoding`
   * `guestinfo.userdata`
   * `guestinfo.userdata.encoding`
   * `disk.enableUUID`
   * `guestinfo.hostname`

   ![Setting up the template, Guest Properties 2](../../images/cloud-provider-vcd/template/GuestProperties2.png)

   ![Setting up the template, Guest Properties 3](../../images/cloud-provider-vcd/template/GuestProperties3.png)

1. In the vCenter management panel for the template, enable the `disk.EnableUUID` parameter:

   ![Setting up the template, vCenter 1](../../images/cloud-provider-vcd/template/vCenter1.png)

   ![Setting up the template, vCenter 2](../../images/cloud-provider-vcd/template/vCenter2.png)

   ![Setting up the template, vCenter 3](../../images/cloud-provider-vcd/template/vCenter3.png)

   ![Setting up the template, vCenter 4](../../images/cloud-provider-vcd/template/vCenter4.png)

   ![Setting up the template, vCenter 5](../../images/cloud-provider-vcd/template/vCenter5.png)

## Using the storage

* VCD supports CSI; disks are created as VCD Independent Disks.
* The `disk.EnableUUID` guest property must be set for the virtual machine templates in use.
* Deckhouse Kubernetes Platform supports disk resizing as of v1.59.1.
