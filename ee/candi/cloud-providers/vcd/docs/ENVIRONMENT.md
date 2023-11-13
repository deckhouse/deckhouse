---
title: "Cloud provider — Vcloud Director: Preparing environment"
description: "Configuring Vcloud Director for Deckhouse cloud provider operation."
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

[](../../images/030-cloud-provider-vcd/network-setup/Screenshot.png)
[](../../images/030-cloud-provider-vcd/network-setup/Screenshot2.png)
[](../../images/030-cloud-provider-vcd/network-setup/Screenshot3.png)
[](../../images/030-cloud-provider-vcd/network-setup/Screenshot4.png)
[](../../images/030-cloud-provider-vcd/network-setup/Screenshot5.png)
[](../../images/030-cloud-provider-vcd/network-setup/Screenshot6.png)

### Adding vApp

[](../../images/030-cloud-provider-vcd/application-setup/Screenshot.png)
[](../../images/030-cloud-provider-vcd/application-setup/Screenshot2.png)

### Adding internal network to vApp

[](../../images/030-cloud-provider-vcd/network-in-vapp-setup/Screenshot.png)
[](../../images/030-cloud-provider-vcd/network-in-vapp-setup/Screenshot2.png)

### Setup DNAT rules on EDGE gateway

[](../../images/030-cloud-provider-vcd/edge-gateway-setup/Screenshot.png)
[](../../images/030-cloud-provider-vcd/edge-gateway-setup/Screenshot2.png)

## Catalog

* You can add distro's cloud images ([Ubuntu](https://cloud-images.ubuntu.com/) for example) to Catalog and use them on the machine creation.
* Cloud-init support should be in the cloud image.

### Inbound traffic

* You can DNAT incoming traffic on the EDGE router (ports 80, 443) to the specific ip address in the internal network.
* This ip address is managed by MetalLB in L2 mode on the dedicated frontend nodes.

### Using the data store

* VCD supports CSI, disks created by CSI is VCD Independent Disks.
* Known limitation - CSI disks cannot support resizing at all.
