---
title: Chaos engineering
permalink: en/admin/configuration/high-reliability-and-availability/chaos-engineering.html
description: Cluster fault tolerance testing
---

{% alert level="warning" %}
Chaos engineering mode can only be enabled for node groups with [`nodeType: CloudEphemeral`](/modules/node-manager/cr.html#nodegroup-v1-spec-nodetype).
{% endalert %}

Enable chaos engineering mode for a node group in one of the following ways:

1. Define the [`spec.chaos`](/modules/node-manager/cr.html#nodegroup-v1-spec-chaos) parameter in the configuration of the target node group with two nested parameters:

   ```yaml
   chaos:
     mode: DrainAndDelete
     period: 24h
   ```

   Where:

   - `mode`: Operation mode. Two options are available:
     - `DrainAndDelete`: When triggered, drains the node and deletes it.
     - `Disabled`: Skips this specific node group.
   - `period`: Chaos Monkey trigger interval. Defined as a string with hours and minutes: `30m`, `1h`, `2h30m`, `24h`.

   Example configuration for a node group:

   ```yaml
   # NodeGroup for cloud nodes in AWS.
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: test
   spec:
     nodeType: CloudEphemeral
     chaos:
       mode: DrainAndDelete
       period: 24h
   ```

1. If the [`console`](/modules/console/) module is enabled in the cluster,
  open the Deckhouse web UI, go to the settings of the desired node group under **Nodes** — **Node Groups**,
  and enable Chaos Monkey in the **Chaos monkey settings** section
  by specifying the time intervals in the corresponding fields.
