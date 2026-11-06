---
title: CloudStatic node
description: A node created manually or by external tools in the cloud for which the provider integration is configured, and connected to the cluster with the bootstrap script. CSI runs on it, and cloud-controller-manager manages its Node object.
lang: en
---

The Node object of a CloudStatic node automatically receives the zone and region from the provider data. If the machine is deleted from the cloud, its Node object is deleted from the cluster. Like a [static node](glossary.html#static-node), a CloudStatic node does not support automatic scaling and updates.

For details, see [Adding and managing cloud nodes](admin/configuration/platform-scaling/node/cloud-node.html).
