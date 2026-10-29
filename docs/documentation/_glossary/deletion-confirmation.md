---
title: Deletion confirmation
description: A confirmation that the user must give in the web interface before an application is deleted or a module is disabled. It is declared in the disable section of package.yaml or module.yaml together with the message shown to the user.
lang: en
---

For a [Marketplace package](glossary.html#marketplace-package) the `disable` section sets `confirmation: true` and localized messages, for example a warning that deleting the application deletes the data stored in its volumes. Deckhouse Platform publishes the section in the `status.packageMetadata.disableOptions` field of the package version, and the [Deckhouse web UI](glossary.html#web-ui) shows the message in the language of the user. The confirmation is not checked when an Application is deleted through the Kubernetes API.

For a [module](glossary.html#module) the `disable.confirmation` and `disable.message` parameters of `module.yaml` work the same way when the module is disabled.

For details, see [Application development](architecture/marketplace/application-development.html).
