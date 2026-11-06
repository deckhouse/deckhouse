---
title: Main cluster network
description: The network in which every virtual machine gets an IP address. It is bound to the main network interface of the pod that runs the machine.
lang: en
---

In the list of networks of a [virtual machine](glossary.html#virtual-machine) the main cluster network has the type `Main`. It does not have to be listed, but if it is, it must come first. Adding or removing the main network requires a reboot of the machine, unlike [additional networks](glossary.html#additional-network), because it cannot be changed on a running pod. The subnets that machines get addresses from are set in `virtualMachineCIDRs` of the [`virtualization`](/modules/virtualization/) module. When an address pool is configured for the main network in the [`sdn`](/modules/sdn/) module, the addresses are managed by the shared [IPAM](glossary.html#sdn-ipam).

For details, see [IP addresses of virtual machines](user/network/virtualization/vm-ip-addresses.html).
