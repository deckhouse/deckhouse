---
title: "Cloud provider — VMware Cloud Director: Preparing environment"
description: "Configuring VMware Cloud Director for Deckhouse cloud provider operation."
---

<!-- AUTHOR! Don't forget to update getting started if necessary -->

## List of required VCD resources

* **Organization**
* **VirtualDataCenter**
* **StoragePolicy**
* **SizingPolicy**
* **Network**
* **EdgeRouter**
* **Catalog**

### Adding network

Create internal network and connect it to Edge Gateway.

![Adding network, step 1](../../images/030-cloud-provider-vcd/network-setup/Screenshot.png)
![Adding network, step 2](../../images/030-cloud-provider-vcd/network-setup/Screenshot2.png)
![Adding network, step 3](../../images/030-cloud-provider-vcd/network-setup/Screenshot3.png)
![Adding network, step 4](../../images/030-cloud-provider-vcd/network-setup/Screenshot4.png)
![Adding network, step 5](../../images/030-cloud-provider-vcd/network-setup/Screenshot5.png)
![Adding network, step 6](../../images/030-cloud-provider-vcd/network-setup/Screenshot6.png)

### Adding vApp

![Adding vApp step 1](../../images/030-cloud-provider-vcd/application-setup/Screenshot.png)
![Adding vApp step 2](../../images/030-cloud-provider-vcd/application-setup/Screenshot2.png)

### Adding internal network to vApp

![Adding internal network to vApp, step 1](../../images/030-cloud-provider-vcd/network-in-vapp-setup/Screenshot.png)
![Adding internal network to vApp, step 2](../../images/030-cloud-provider-vcd/network-in-vapp-setup/Screenshot2.png)

### Setup DNAT rules on EDGE gateway

![Setup DNAT rules on EDGE gateway, step 1](../../images/030-cloud-provider-vcd/edge-gateway-setup/Screenshot.png)
![Setup DNAT rules on EDGE gateway, step 2](../../images/030-cloud-provider-vcd/edge-gateway-setup/Screenshot2.png)

## Catalog

* You can add distro's cloud images ([Ubuntu](https://cloud-images.ubuntu.com/) for example) to Catalog and use them on the machine creation.
* Cloud-init support should be in the cloud image.

### Inbound traffic

* You can DNAT incoming traffic on the EDGE router (ports 80, 443) to the specific ip address in the internal network.
* This ip address is managed by MetalLB in L2 mode on the dedicated frontend nodes.

### Using the data store

* VCD supports CSI, disks created by CSI is VCD Independent Disks.
* Guest property `disk.EnableUUID` should be enabled for the used machine templates.
* Known limitation - CSI disks cannot support resizing at all.
