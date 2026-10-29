---
title: Release channel
description: One of the five channels, from Alpha to Rock Solid, through which every new Deckhouse Platform version is rolled out gradually. The cluster receives the versions published on the channel it follows.
lang: en
---

Each new version is first published to the Alpha channel and then moves to Beta, Early Access, Stable and Rock Solid. Versions appear on Early Access at least 7 days after the release, on Stable at least 14 days and on Rock Solid at least 30 days after the release.

The channel of the cluster is set in the `settings.releaseChannel` parameter of the [`deckhouse`](/modules/deckhouse/) [module](glossary.html#module). How a new version is applied depends on the [update mode](glossary.html#update-mode) and [update windows](glossary.html#update-window).

For details, see [Updating](architecture/updating.html).
