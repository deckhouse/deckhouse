---
title: Maintenance mode of a module or application
description: "A mode in which Deckhouse Platform stops applying templates and restoring the resources of a module or an application, so that they can be changed manually. It is enabled with spec.maintenance: NoResourceReconciliation and is not related to the node maintenance mode."
lang: en
---

In this mode changes of settings and values are not reflected in the resources, deleted resources are not restored, and hooks keep running. The resources rendered from the templates get the `maintenance.deckhouse.io/no-resource-reconciliation` label, and the [admission policies](glossary.html#admission-policy) stop protecting them. For an [application instance](glossary.html#application-instance) the `Managed` condition becomes `False` and the `ApplicationIsInMaintenanceMode` alert fires. To return to normal operation, remove the `spec.maintenance` field.

The field is set in [ModuleConfig](reference/api/cr.html#moduleconfig) for a [module](glossary.html#module) and in [Application](reference/api/cr.html#application) for an application. The mode is intended for debugging and development, not for production clusters. The [node maintenance mode](glossary.html#maintenance-mode) is a different mechanism.

For details, see [Lifecycle and debugging](architecture/marketplace/lifecycle.html).
