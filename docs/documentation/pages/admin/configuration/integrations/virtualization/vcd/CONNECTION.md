---
title: Connection and authorization
permalink: en/admin/integrations/virtualization/vcd/connection-and-authorization.html
---

## Resource preparation

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

To manage resources in VCD using the "Deckhouse Kubernetes Platform", the following resources must be configured in the system:

* Organization
* VirtualDataCenter
* vApp (for the "Standard" placement scheme)
* StoragePolicy
* SizingPolicy
* Network (for the "Standard" placement scheme)
* EdgeRouter
* Catalog

The Organization, VirtualDataCenter, StoragePolicy, SizingPolicy, EdgeRouter, and Catalog resources must be provided by your VMware Cloud Director service provider.

The Network (internal network) can be configured by your VMware Cloud Director service provider or by yourself. When using the "StandardWithNetwork" placement scheme, the network is created automatically. Below is a method for manually setting up an internal network.

### User permissions

The user accessing the VMware Cloud Director API must have the following permissions:

* Role "Organization Administrator" with an additional rule "Preserve All ExtraConfig Elements During OVF Import and Export";
* The "Preserve All ExtraConfig Elements During OVF Import and Export" rule must also be included in the user’s "Right Bundle".

### Adding a network

{% alert level="info" %}
This instruction applies only to the "Standard" placement scheme.
{% endalert %}

1. Go to the "Networking" tab and click "NEW":

   ![Add network, step 1](../../../../images/cloud-provider-vcd/network-setup/Screenshot.png)

2. Select the desired "Data Center":

   ![Add network, step 2](../../../../images/cloud-provider-vcd/network-setup/Screenshot2.png)

3. In the "Network type" step, select "Routed":

   ![Add network, step 3](../../../../images/cloud-provider-vcd/network-setup/Screenshot3.png)

4. Connect the "EdgeRouter" to the network:

   ![Add network, step 4](../../../../images/cloud-provider-vcd/network-setup/Screenshot4.png)

5. Specify the network name and CIDR:

   ![Add network, step 5](../../../../images/cloud-provider-vcd/network-setup/Screenshot5.png)

6. Do not add "Static IP Pools" since DHCP will be used:

   ![Add network, step 6](../../../../images/cloud-provider-vcd/network-setup/Screenshot6.png)

7. Specify DNS server addresses:

   ![Add network, step 7](../../../../images/cloud-provider-vcd/network-setup/Screenshot7.png)

### DHCP setup

{% alert level="info" %}
This instruction applies only to the "Standard" placement scheme.
{% endalert %}

To dynamically provision nodes, enable the DHCP server for the internal network.

{% alert level="info" %}
We recommend reserving the beginning of the address range for system workloads (control plane, frontend nodes, system nodes), and using the rest for the DHCP pool.  
For example, for a "/24" network, reserving 20 addresses for system workloads is sufficient.
{% endalert %}

1. Go to the "Networking" tab and open the created network:

   ![DHCP, step 1](../../../../images/cloud-provider-vcd/dhcp-setup/Screenshot.png)

2. In the opened window, select "IP Management" → "DHCP" → "Activate":

   ![DHCP, step 2](../../../../images/cloud-provider-vcd/dhcp-setup/Screenshot2.png)

3. In the "General settings" tab, configure parameters as shown in the example:

   ![DHCP, step 3](../../../../images/cloud-provider-vcd/dhcp-setup/Screenshot3.png)

4. Add a pool:

   ![DHCP, step 4](../../../../images/cloud-provider-vcd/dhcp-setup/Screenshot4.png)

5. Specify DNS server addresses:

   ![DHCP, step 5](../../../../images/cloud-provider-vcd/dhcp-setup/Screenshot5.png)

### Adding a vApp

{% alert level="info" %}
This instruction applies only to the "Standard" placement scheme.
{% endalert %}

1. Go to "Data Centers" → "vApps" → "NEW" → "New vApp":

   ![Add vApp, step 1](../../../../images/cloud-provider-vcd/application-setup/Screenshot.png)

2. Specify a name and enable the vApp:

   ![Add vApp, step 2](../../../../images/cloud-provider-vcd/application-setup/Screenshot2.png)

### Adding a network to a vApp

{% alert level="info" %}
This instruction applies only to the "Standard" placement scheme.
{% endalert %}

After creating the vApp, attach the created internal network to it.

1. Go to "Data Centers" → "vApps" and open the desired vApp:

   ![Add network to vApp, step 1](../../../../images/cloud-provider-vcd/network-in-vapp-setup/Screenshot.png)

2. Go to the "Networks" tab and click "NEW":

   ![Add network to vApp, step 2](../../../../images/cloud-provider-vcd/network-in-vapp-setup/Screenshot2.png)

3. In the pop-up window, select "Direct" type and choose the network:

   ![Add network to vApp, step 3](../../../../images/cloud-provider-vcd/network-in-vapp-setup/Screenshot3.png)

### Incoming traffic

Incoming traffic must be directed to the edge router (ports "80", "443") using DNAT rules to the allocated address in the internal network.  
This address is managed by MetalLB in L2 mode on dedicated frontend nodes.

### Configuring DNAT/SNAT rules on the Edge Gateway

1. Go to "Networking" → "Edge Gateways", open the edge gateway:

   ![DNAT setup, step 1](../../../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot.png)

2. Go to "Services" → "NAT":

   ![DNAT setup, step 2](../../../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot2.png)

3. Add the following rules:

   ![DNAT setup, step 3](../../../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot3.png)

   The first two rules are for incoming traffic, and the third is for SSH access to the control plane node (required for installation).

4. To allow virtual machines to access the internet, configure SNAT rules as in the example:

   ![SNAT setup, step 1](../../../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot4.png)

   This rule allows VMs from the "192.168.199.0/24" subnet to access the internet.

### Firewall setup

{% alert level="info" %}
This instruction applies only to the "Standard" placement scheme.
{% endalert %}

After configuring DNAT, configure the firewall. Start by setting up IP sets.

1. Go to "Security" → "IP Sets":

   ![Firewall setup, step 1](../../../../images/cloud-provider-vcd/edge-firewall/Screenshot.png)

2. Create the following IP set (assuming the MetalLB address will be ".10" and the control plane node ".2"):

   ![Firewall setup, step 2-1](../../../../images/cloud-provider-vcd/edge-firewall/Screenshot2.png)
   ![Firewall setup, step 2-2](../../../../images/cloud-provider-vcd/edge-firewall/Screenshot3.png)
   ![Firewall setup, step 2-3](../../../../images/cloud-provider-vcd/edge-firewall/Screenshot4.png)

3. Add the following firewall rules:

   ![Firewall setup, step 3](../../../../images/cloud-provider-vcd/edge-firewall/Screenshot5.png)

## Virtual machine template

{% alert level="warning" %}
The provider has been tested only with virtual machine templates based on "Ubuntu 22.04".
{% endalert %}

{% include notice_envinronment.liquid %}

In the example, an "OVA" file provided by Ubuntu is used, with two modifications.  
These modifications are required for proper provisioning of CloudPermanent nodes and to allow attaching disks created by CSI.

### Preparing the template from an OVA file

1. Download [the OVA file](https://cloud-images.ubuntu.com/jammy/):

   ![Template setup, step 1](../../../../images/cloud-provider-vcd/template/Screenshot.png)

2. Go to "Libraries" → "Catalogs" → "Organization Catalog":

   ![Template setup, step 2](../../../../images/cloud-provider-vcd/template/Screenshot2.png)

3. Upload the downloaded template to the catalog:

   ![Template setup, step 3](../../../../images/cloud-provider-vcd/template/Screenshot3.png)
   ![Template setup, step 4](../../../../images/cloud-provider-vcd/template/Screenshot4.png)
   ![Template setup, step 5](../../../../images/cloud-provider-vcd/template/Screenshot5.png)

4. Create a VM from the template:

   ![Template setup, step 6](../../../../images/cloud-provider-vcd/template/Screenshot6.png)
   ![Template setup, step 7](../../../../images/cloud-provider-vcd/template/Screenshot7.png)

{% alert level="warning" %}
Set a default password and public key. These will be needed to log into the VM console.
{% endalert %}

![Template setup, step 8](../../../../images/cloud-provider-vcd/template/Screenshot8.png)

To connect to the VM:

1. Start the VM.
2. Wait until it gets an IP address.
3. Forward port "22" to the VM:

   ![Template setup, step 9](../../../../images/cloud-provider-vcd/template/Screenshot9.png)

Log into the VM via SSH and run:

```shell
rm /etc/netplan/99-netcfg-vmware.yaml
echo -e '\n[deployPkg]\nwait-cloudinit-timeout=1800\n' >> /etc/vmware-tools/tools.conf
echo 'disable_vmware_customization: true' > /etc/cloud/cloud.cfg.d/91_vmware_cust.cfg
dpkg-reconfigure cloud-init
```

In the dialog window, leave only the checkbox for `OVF: Reads data from OVF transports`. Disable all other options:

![Template setup, OVF](../../../../images/cloud-provider-vcd/template/OVF.png)

Additionally, make sure that the `datasource_list` parameter is specified in the cloud-init configuration. You can verify this using the following command:

```shell
cat /etc/cloud/cloud.cfg.d/90_dpkg.cfg
```

If the output is empty, execute the following command:

```shell
echo "datasource_list: [ OVF, VMware, None ]" > /etc/cloud/cloud.cfg.d/90_dpkg.cfg
```

Run the remaining commands:

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

### Configuring the template in VCD

1. Power off the virtual machine and delete all filled "Guest Properties" fields:

   ![Template setup, Guest Properties 1](../../../../images/cloud-provider-vcd/template/GuestProperties1.png)

   ![Template setup, Guest Properties 5](../../../../images/cloud-provider-vcd/template/GuestProperties5.png)

1. Create a virtual machine template:

   ![Template setup, step 10](../../../../images/cloud-provider-vcd/template/Screenshot10.png)

   ![Template setup, step 11](../../../../images/cloud-provider-vcd/template/Screenshot11.png)

1. In the created template, go to the "Metadata" tab and add six fields:

   * `guestinfo.metadata`
   * `guestinfo.metadata.encoding`
   * `guestinfo.userdata`
   * `guestinfo.userdata.encoding`
   * `disk.enableUUID`
   * `guestinfo.hostname`

   ![Template setup, Guest Properties 2](../../../../images/cloud-provider-vcd/template/GuestProperties2.png)

   ![Template setup, Guest Properties 3](../../../../images/cloud-provider-vcd/template/GuestProperties3.png)

1. In the vCenter management panel, enable the `disk.EnableUUID` parameter for the template:

   ![Template setup, vCenter 1](../../../../images/cloud-provider-vcd/template/vCenter1.png)

   ![Template setup, vCenter 2](../../../../images/cloud-provider-vcd/template/vCenter2.png)

   ![Template setup, vCenter 3](../../../../images/cloud-provider-vcd/template/vCenter3.png)

   ![Template setup, vCenter 4](../../../../images/cloud-provider-vcd/template/vCenter4.png)

   ![Template setup, vCenter 5](../../../../images/cloud-provider-vcd/template/vCenter5.png)

## Storage usage

* VCD supports CSI. Disks are created as VCD Independent Disks.
* The guest property `disk.EnableUUID` must be enabled for the VM templates in use.
* Deckhouse Kubernetes Platform supports disk resizing starting from version v1.59.1.
