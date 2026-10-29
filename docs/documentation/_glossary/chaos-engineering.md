---
title: Chaos engineering
description: Deliberate or random disruption of cluster components to test its resilience. In Deckhouse Platform it is enabled per node group with the [spec.chaos](/modules/node-manager/cr.html#nodegroup-v1-spec-chaos) parameter, only for CloudEphemeral nodes.
lang: en
---

The `spec.chaos` parameter of a [node group](glossary.html#node-group) of [CloudEphemeral nodes](glossary.html#cloud-ephemeral-node) sets the mode and the period. In `DrainAndDelete` mode Chaos Monkey drains and deletes a node of the group once per `period`. The `Disabled` mode skips the group. The mode can also be enabled in the web interface in the node group settings.

For details, see [Chaos engineering](admin/configuration/high-reliability-and-availability/chaos-engineering.html).
