---
title: Update window
description: A time interval during which Deckhouse Platform is allowed to install updates automatically. Outside the windows new versions are not installed.
lang: en
---

Update windows are set in the `update.windows` parameter of the [`deckhouse`](/modules/deckhouse/) [module](glossary.html#module). If no windows are configured, an update is applied as soon as a new version appears on the [release channel](glossary.html#release-channel).

[Disruptive updates](glossary.html#disruptive-update) of nodes have separate windows in the [NodeGroup](/modules/node-manager/cr.html#nodegroup) parameters `disruptions.automatic.windows` and `disruptions.rollingUpdate.windows`.

For details, see [Configuring updates](admin/configuration/update/configuration.html).
