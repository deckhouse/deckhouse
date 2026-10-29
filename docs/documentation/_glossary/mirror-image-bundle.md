---
title: Deckhouse Platform image bundle
description: An archive of Deckhouse Platform container images produced by d8 mirror pull and loaded into a private container registry with d8 mirror push. It is the way to deliver platform images into a private environment.
lang: en
---

The bundle is downloaded from the public Deckhouse Platform registry with a [license key](glossary.html#license-key), usually on the [bastion host](glossary.html#bastion-host), and brought into the perimeter by any permitted means, for example on removable media. Its size after `d8 mirror push` guides the disk capacity of the private registry. The `d8 mirror` commands are not available in the Community Edition and Basic Edition [editions](glossary.html#edition).

For details, see [Installing DP in a private environment](guides/private-environment.html).
