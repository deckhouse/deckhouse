---
title: Instance class
description: A provider-specific resource, such as OpenStackInstanceClass or AWSInstanceClass, that describes the parameters of the virtual machines Deckhouse Platform creates for a node group.
lang: en
---

A [node group](glossary.html#node-group) of [CloudEphemeral nodes](glossary.html#cloud-ephemeral-node) refers to an instance class in `cloudInstances.classReference`. The instance class defines the machine type, the image, disks and other provider-specific parameters, while the number of nodes and their zones are defined by the node group.

For details, see [Adding and managing cloud nodes](admin/configuration/platform-scaling/node/cloud-node.html).
