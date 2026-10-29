---
title: Update mode
description: The setting that defines how Deckhouse Platform applies new versions.
lang: en
---

In the `Auto` mode both patch and minor versions are applied automatically, in the `AutoPatch` mode only patch versions within the current minor version. In the `Manual` mode a version is applied only after the administrator approves the corresponding [DeckhouseRelease](reference/api/cr.html#deckhouserelease).

The mode is set in the `settings.update.mode` parameter of the [`deckhouse`](/modules/deckhouse/) [module](glossary.html#module). In the automatic modes updates are applied with regard to [update windows](glossary.html#update-window).

For details, see [Configuring updates](admin/configuration/update/configuration.html).
