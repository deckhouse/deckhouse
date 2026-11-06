---
title: Application suspension
description: The state of an installed application whose package requirements have stopped being met, for example a mandatory module was disabled. Deckhouse Platform uninstalls the Helm release but keeps the [Application](reference/api/cr.html#application) in the Suspended state.
lang: en
---

Deckhouse Platform checks the package requirements all the time, not only on admission. When they stop being met, it runs the `beforeDeleteHelm` hooks, uninstalls the Helm release, which deletes the resources of the [application instance](glossary.html#application-instance), and runs the `afterDeleteHelm` hooks. The [orphan resources](glossary.html#application-orphan-resources) are not deleted. When the requirements are met again, the application is installed again with the same settings.

The resources are deleted together with the Helm release, including PersistentVolumeClaims. To keep the data, add the `helm.sh/resource-policy: keep` annotation to them.

For details, see [Lifecycle and debugging](architecture/marketplace/lifecycle.html).
