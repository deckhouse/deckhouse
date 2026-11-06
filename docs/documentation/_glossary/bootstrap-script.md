---
title: Bootstrap script
description: The bootstrap.sh script that Deckhouse Platform generates for every [NodeGroup](/modules/node-manager/cr.html#nodegroup) to connect a server to the cluster as a node of that group.
lang: en
---

The script is stored in the `manual-bootstrap-for-<NODE-GROUP>` secret in the `d8-cloud-instance-manager` namespace and is also available in the Deckhouse web interface. For manual connection of a [static node](glossary.html#static-node), copy the script to the server and run it as `root`. [CAPS](glossary.html#caps) runs it over SSH itself. The script installs and configures the operating system settings, containerd, kubelet and the `kubernetes-api-proxy` component, then registers the node in the cluster.

For details, see [Adding and managing bare-metal nodes](admin/configuration/platform-scaling/node/bare-metal-node.html).
