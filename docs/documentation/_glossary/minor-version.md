---
title: Minor version
description: A Deckhouse Platform release that includes new features.
lang: en
---

A minor version is one of the two types of a [Deckhouse Platform release](glossary.html#dp-release), the other being a [patch version](glossary.html#patch-version). Minor versions are installed sequentially: if the cluster lags behind the [release channel](glossary.html#release-channel) by several minor versions, Deckhouse Platform reconstructs the intermediate releases, because they contain migrations that must be applied in order. In the `Manual` [update mode](glossary.html#update-mode) a minor version needs approval.

For details, see [Updating](architecture/updating.html).
