---
title: SDN IPAM
description: IP address management of the sdn module. It allocates IPv4 addresses from pools and assigns them to the additional network interfaces of pods and virtual machines. With a pool for the main cluster network it also manages the addresses of virtual machines there.
lang: en
---

IPAM is enabled per network through the address pool reference in Network or [ClusterNetwork](/modules/sdn/cr.html#clusternetwork). Addresses for [cluster networks](glossary.html#cluster-network) are allocated by the cluster administrator from cluster-wide pools. In [project networks](glossary.html#project-network) users create their own pools and may request a specific address for an interface. If several networks with IPAM are connected to one pod, specify the address objects for each interface explicitly.

When a pool is configured for the [main cluster network](glossary.html#main-network), the addresses of [virtual machines](glossary.html#virtual-machine) in that network are managed by the shared IPAM: the address is requested automatically, and existing machines switch to it without changing their addresses and without a restart.

For details, see [Additional networks for use in application pods](user/network/sdn/dedicated.html).
