---
title: Marketplace package
description: "An abstract unit of Marketplace: either an [application](reference/api/cr.html#application), a user workload deployed into a namespace, or a module that extends the capabilities of the cluster."
lang: en
---

Packages are published in container registries connected to [Marketplace](glossary.html#marketplace) as package repositories. For every discovered package version Deckhouse Platform stores its metadata: description, [lifecycle stage](glossary.html#module-lifecycle-stage), requirements, [deletion confirmation](glossary.html#deletion-confirmation), changelog and the OpenAPI schemas of settings and values.

For details, see [Concepts](architecture/marketplace/concepts.html).
