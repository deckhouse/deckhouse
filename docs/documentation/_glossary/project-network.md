---
title: Project network
description: An additional software-defined network available within one namespace. The user creates and manages it with the [NetworkClass](/modules/sdn/cr.html#networkclass) provided by the administrator. It is also called a user network.
lang: en
---

The administrator defines in a NetworkClass the range of VLAN tags and the node network interfaces on which users may create their own networks. The user creates the network with the Network resource in the [namespace](glossary.html#namespace) of the [project](glossary.html#project). Users can allocate an address pool for the project network and assign addresses to pod interfaces through [IPAM](glossary.html#sdn-ipam). The other kind of [additional network](glossary.html#additional-network) is the [cluster network](glossary.html#cluster-network).

For details, see [Additional networks for use in application pods](user/network/sdn/dedicated.html).
