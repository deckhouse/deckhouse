---
title: Module accessibility
description: The accessibility settings of module.yaml that define in which Deckhouse Platform editions a module is available and in which module bundles it is enabled by default.
lang: en
---

Settings are given per [edition](glossary.html#edition) with a `_default` block for editions without their own settings: `available` says whether the module can be used in the edition, `enabledInBundles` lists the [module bundles](glossary.html#module-bundle) (`Default`, `Managed`, `Minimal`) in which the module is enabled without a [ModuleConfig](reference/api/cr.html#moduleconfig).

For details, see [Module structure](architecture/module-development/structure/).
