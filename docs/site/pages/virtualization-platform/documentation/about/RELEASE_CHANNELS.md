---
title: "Release channels"
permalink: en/virtualization-platform/documentation/about/release-channels.html
lang: en
---

Deckhouse Virtualization Platform (DVP) uses five release channels intended for use in different environments, with varying requirements in terms of stability:

| Release channel | Description                                                                                                                                                                                                                                                                                          |
|-----------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Alpha           | The least stable release channel but introduces new versions most frequently. Intended for development clusters used by a small number of developers.                                                                                                                            |
| Beta            | Intended for development clusters, similar to the Alpha channel. Receives versions that have been previously tested on the Alpha channel.                                                                                                                                                       |
| Early Access    | The recommended release channel if you are unsure which channel to use. Suitable for clusters with active development, such as running applications or debugging. New features are introduced to this channel no earlier than one week after their initial release. |
| Stable          | The stable release channel for clusters where active development has finished and the focus is on normal operation. New features are introduced to this channel no earlier than two weeks after their initial release.                                                |
| Rock Solid      | The most stable release channel. Suitable for clusters that require a higher level of stability. New features are introduced to this channel no earlier than one month after their initial release.                                                                 |

DVP components can update either automatically or upon manual confirmation as updates are released in the release channels.

For information about the versions available in release channels, visit [releases.deckhouse.io](https://releases.deckhouse.io/).

For details on configuring release channels, refer to [Platform update](../admin/update/update.html).
