---
title: Hybrid cluster
description: A static Deckhouse Platform cluster extended with nodes placed in the infrastructure of a supported cloud provider.
lang: en
---

The control plane and the initial nodes remain part of the [static cluster](glossary.html#static-cluster). Additional nodes are added through the [cloud provider](glossary.html#cloud-provider) module: either created automatically through the provider API as [CloudEphemeral nodes](glossary.html#cloud-ephemeral-node), or created in advance and connected with the [bootstrap script](glossary.html#bootstrap-script) as [CloudStatic nodes](glossary.html#cloud-static-node).

A hybrid cluster requires L3 connectivity between the data center and the cloud.

For details, see [Hybrid clusters](admin/integrations/hybrid/overview.html).
