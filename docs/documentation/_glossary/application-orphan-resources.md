---
title: Orphan resources of an application
description: Objects that stay in the namespace after the Helm release of an application is uninstalled, for example objects created by workloads or hooks at runtime. Deckhouse Platform deletes the listed kinds together with the [Application](reference/api/cr.html#application) object.
lang: en
---

The package declares the kinds of orphan resources in the `orphanResources` field of `openapi/values.yaml`. When the Application is deleted, Deckhouse Platform deletes the objects of these kinds in the namespace of the [application instance](glossary.html#application-instance) that carry the `packages.deckhouse.io/instance: <INSTANCE_NAME>` label, after the Helm release is uninstalled and the hooks are stopped. Objects created by workloads and hooks do not get the label automatically.

Orphan resources are deleted only when the Application is deleted. On [suspension](glossary.html#application-suspension) and on a package version change they stay in the cluster.

For details, see [Lifecycle and debugging](architecture/marketplace/lifecycle.html).
