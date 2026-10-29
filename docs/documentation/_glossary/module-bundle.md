---
title: Module bundle
description: A named set of built-in Deckhouse Platform modules that are enabled by default. Default, Minimal or Managed.
lang: en
---

The bundle is set in the `bundle` parameter of the [`deckhouse`](/modules/deckhouse/) [module](glossary.html#module) and cannot be changed in a deployed cluster. `Default` is the recommended set for cluster operation, including monitoring, authorization, networking and other basic modules. `Managed` is the set for managed clusters of cloud providers. `Minimal` is the minimal set.

Regardless of the bundle, a module can be enabled or disabled explicitly in its [ModuleConfig](reference/api/cr.html#moduleconfig).

For details, see [Deckhouse Platform configuration](admin/configuration/).
