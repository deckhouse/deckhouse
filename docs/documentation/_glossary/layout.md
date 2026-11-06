---
title: Layout
description: The scheme of node placement in the infrastructure of a cloud provider. It affects networking, availability of public IP addresses, outgoing traffic routing and the way nodes are accessed.
lang: en
---

The layout is set in the `layout` parameter of the [ModuleConfig](reference/api/cr.html#moduleconfig) of the [cloud provider](glossary.html#cloud-provider) module. The available layouts depend on the provider, for example `Standard`, `WithoutNAT` and `WithNATInstance` in Yandex Cloud.

In some layouts nodes have no public IP addresses, and the master node is reached through a [bastion host](glossary.html#bastion-host).

For details, see [Integration with cloud providers](admin/integrations/public/overview.html).
