---
title: Deckhouse Platform release
description: Any published version of Deckhouse Platform. Releases are distributed through release channels with defined delays and are of two types, a patch version and a minor version.
lang: en
---

A [patch version](glossary.html#patch-version) includes bug fixes, a [minor version](glossary.html#minor-version) includes new features. Each new release first appears on the Alpha [release channel](glossary.html#release-channel) and then moves to the more stable channels. In the cluster a release is represented by a [DeckhouseRelease](reference/api/cr.html#deckhouserelease) object, which is applied according to the [update mode](glossary.html#update-mode) and [update windows](glossary.html#update-window). Minor releases cannot be skipped, because they contain migrations that must be applied in order.

For details, see [Updating](architecture/updating.html).
