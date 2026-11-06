---
title: IPAM
description: An IP address management mechanism that allocates addresses to the network interfaces of pods and virtual machines and ensures that no address is assigned twice. In Deckhouse Platform the CNI modules, the sdn module and the virtualization module each have their own IPAM.
lang: en
---

The `cni-cilium`, `cni-flannel` and `cni-simple-bridge` modules assign pod addresses from the pod address range allocated to the node. This IPAM is not configurable.

In the [`sdn`](/modules/sdn/) module, IPAM allocates IPv4 addresses from pools to the additional network interfaces of pods and virtual machines. It is enabled for each network separately by an address pool reference in [Network](/modules/sdn/cr.html#network) or [ClusterNetwork](/modules/sdn/cr.html#clusternetwork). For [cluster networks](glossary.html#cluster-network) the pools are created by the administrator, for [project networks](glossary.html#project-network) by users. If a pod is connected to multiple networks that use IPAM, specify the address objects for each interface explicitly.

The addresses of [virtual machines](glossary.html#virtual-machine) in the [main cluster network](glossary.html#main-network) are managed by default by the [`virtualization`](/modules/virtualization/) module through the [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) and [VirtualMachineIPAddressLease](modules/virtualization/cr.html#virtualmachineipaddresslease). When a pool is configured for the main network in the `sdn` module, the addresses of virtual machines are managed by the shared IPAM of that module: the address is requested automatically, and existing virtual machines are brought under its management without changing their addresses or restarting.

For details, see [Additional networks for use in application pods](user/network/sdn/dedicated.html) and [IP addresses of virtual machines](user/network/virtualization/vm-ip-addresses.html).
