---
title: Additional network
description: A software-defined network for pods and virtual machines on top of the main cluster network, provided by the sdn module. Additional networks are cluster networks or project networks.
lang: en
---

A [cluster network](glossary.html#cluster-network) is publicly available in every [project](glossary.html#project) and is managed by the administrator. A [project network](glossary.html#project-network) is available within one [namespace](glossary.html#namespace) and is created by the user. Pods are connected to additional networks with a pod annotation, virtual machines with the list of networks in the machine specification. IP addresses on the additional interfaces are assigned by [SDN IPAM](glossary.html#sdn-ipam).

For details, see [Additional networks for use in application pods](user/network/sdn/dedicated.html).
