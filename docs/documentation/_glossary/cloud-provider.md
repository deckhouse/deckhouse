---
title: Cloud provider
description: A cloud platform or virtualization system that Deckhouse Platform integrates with through a cloud-provider-* module to create virtual machines, connect them to the cluster as nodes and create base infrastructure resources.
lang: en
---

The workflow is the same for all providers, only the preparation steps, such as creating a service account, and the configuration files differ. The integration enables [CloudEphemeral nodes](glossary.html#cloud-ephemeral-node), which Deckhouse Platform creates and deletes through the provider API, and the choice of a [layout](glossary.html#layout).

For details, see [Integration with IaaS](admin/integrations/integrations-overview.html).
