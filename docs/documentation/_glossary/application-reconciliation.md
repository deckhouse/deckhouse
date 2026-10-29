---
title: Application reconciliation run
description: The configuration, hook initialization and apply steps that Deckhouse Platform performs for an application instance. A new run starts when the settings change.
lang: en
---

Deckhouse Platform processes each [application instance](glossary.html#application-instance) in a separate queue: it downloads the [package bundle](glossary.html#marketplace-package-bundle), loads it, waits until the package requirements are met, validates and applies the settings, enables the hook bindings and runs the `onStartup` hooks, then runs `beforeHelm` hooks, renders the templates and applies them with Nelm, and runs `afterHelm` hooks. The last three steps form a reconciliation run. The Helm release is upgraded only when needed: on the first installation, if the rendered manifests have changed, or if some resources are missing in the cluster.

A new run starts when `spec.settings` or `spec.maintenance` of the [Application](reference/api/cr.html#application) changes or when a `schedule` or Kubernetes hook changes values.

For details, see [Lifecycle and debugging](architecture/marketplace/lifecycle.html).
