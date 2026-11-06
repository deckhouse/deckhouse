---
title: Module from a source
description: A module that Deckhouse Platform gets from a module source instead of its own distribution. Its release cycle is not tied to the release cycle of the platform.
lang: en
---

The list of available modules is defined by the [module sources](glossary.html#module-source) tracked by Deckhouse Platform. Such a module is updated by its own [update policy](glossary.html#module-update-policy): it has its own [release channels](glossary.html#release-channel), [lifecycle stage](glossary.html#module-lifecycle-stage) and [dependencies](glossary.html#module-dependencies). Alerts of modules from a source are not included in the alerts reference of the platform.

For details, see [Updating DP in air-gapped environment](guides/airgapped-update.html).
