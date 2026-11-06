---
title: Module source
description: The container registry from which Deckhouse Platform fetches information about modules that are not built into the platform. It is set by [ModuleSource](reference/api/cr.html#modulesource) with the registry address and authentication parameters.
lang: en
---

[Modules from a source](glossary.html#module-from-source) are published to the registry as [module artifacts](glossary.html#module-artifacts). Deckhouse Platform synchronizes with the source and creates module releases according to the [module update policy](glossary.html#module-update-policy). A module is then enabled with [ModuleConfig](reference/api/cr.html#moduleconfig). In an [air-gapped cluster](glossary.html#air-gapped-cluster) the source points to the in-cluster or private registry.

For details, see [How to run and verify a module in the DP cluster](architecture/module-development/run/).
