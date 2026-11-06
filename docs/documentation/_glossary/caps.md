---
title: Cluster API Provider Static (CAPS)
description: The mechanism that connects, configures and, when needed, cleans up static nodes automatically, based on [StaticInstance](/modules/node-manager/cr.html#staticinstance) and [SSHCredentials](/modules/node-manager/cr.html#sshcredentials) objects.
lang: en
---

CAPS is enabled by the `staticInstances` section of a [node group](glossary.html#node-group) with the Static type: `count` sets how many nodes the group must have, `labelSelector` selects the StaticInstance objects to use. CAPS keeps the number of nodes equal to `count`: it connects to a server over SSH, runs the [bootstrap script](glossary.html#bootstrap-script) and adds the node to the cluster, and when a node is removed it cleans the server up and returns its StaticInstance to the `Pending` state for reuse.

For details, see [Adding and managing bare-metal nodes](admin/configuration/platform-scaling/node/bare-metal-node.html).
