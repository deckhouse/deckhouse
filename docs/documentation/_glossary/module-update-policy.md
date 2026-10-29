---
title: Module update policy
description: The rules by which Deckhouse Platform updates a module from a source in the cluster. It is set by [ModuleUpdatePolicy](reference/api/cr.html#moduleupdatepolicy) and defines the update mode, the release channel and the update windows of the module.
lang: en
---

A policy sets the [update mode](glossary.html#update-mode) (automatic, manual or updates disabled), the [release channel](glossary.html#release-channel) used for updates and the [update windows](glossary.html#update-window) in which automatic updates are allowed. If no policy is defined for a module, the module follows the update settings of the platform from the `update` parameter of the [`deckhouse`](/modules/deckhouse/) module. A module release in the `Pending` status waits for manual approval.

For details, see [How to run and verify a module in the DP cluster](architecture/module-development/run/).
